package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
)

// Entry is one command from the Runbookfile.
type Entry struct {
	Name string            // slashes group entries into folders, e.g. "services/api"
	Run  string            // shell command
	Dir  string            // optional working directory, relative to the Runbookfile's directory
	Env  map[string]string // optional extra environment variables, nil if none
}

const (
	fieldRun = "run"
	fieldDir = "dir"
	fieldEnv = "env"
)

// This function parses the Runbookfile and returns a list of entries.
// path: name of the command
//   run: shell command to run
//   dir: optional working directory, relative to the Runbookfile's directory, defaults to Runbookfile's directory
//   env: optional extra environment variables, nil if none
//     key: value
//
// Example Runbookfile
//
// services/api:
//   run: go run main.go
//   dir: ./api
//   env:
//     PORT: 8080
//     DATABASE_URL: postgres://user:password@localhost:5432/db
func parseRunbookfile(r io.Reader) ([]Entry, error) {
	var (
		entries []Entry
		lines   = map[string]int{} // command name -> line it was opened on

		current  *Entry
		headerAt int

		fieldIndent   = -1
		envIndent     = -1
		envPairIndent = -1
	)

	closeCurrent := func() error {
		if current == nil {
			return nil
		}
		if current.Run == "" {
			return fmt.Errorf("line %d: %q has no %s command", headerAt, current.Name, fieldRun)
		}
		entries = append(entries, *current)
		current = nil
		fieldIndent, envIndent, envPairIndent = -1, -1, -1
		return nil
	}

	scanner := bufio.NewScanner(r)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimRight(scanner.Text(), " \t\r")
		body := strings.TrimLeft(line, " \t")
		if body == "" || strings.HasPrefix(body, "#") {
			continue
		}
		space := line[:len(line)-len(body)]
		if strings.ContainsRune(space, '\t') {
			return nil, fmt.Errorf("line %d: indent with spaces, not tabs", n)
		}
		indent := len(space)

		key, value, ok := strings.Cut(body, ":")
		if !ok {
			return nil, fmt.Errorf("line %d: expected \"name:\", got %q", n, body)
		}
		value = strings.TrimSpace(value)

		if indent == 0 {
			if err := closeCurrent(); err != nil {
				return nil, err
			}
			name := key // checked, not trimmed: a name carries no stray spaces
			if err := checkName(name); err != nil {
				return nil, fmt.Errorf("line %d: %w", n, err)
			}
			if value != "" {
				return nil, fmt.Errorf("line %d: %q takes no command here, put it in a %s field below", n, name, fieldRun)
			}
			if at, dup := lines[name]; dup {
				return nil, fmt.Errorf("line %d: %q is already defined on line %d", n, name, at)
			}
			lines[name] = n

			current, headerAt = &Entry{Name: name}, n
			continue
		}

		key = strings.TrimSpace(key)
		if current == nil {
			return nil, fmt.Errorf("line %d: indented %q does not belong to a command", n, key)
		}

		if envIndent >= 0 && indent > envIndent {
			switch {
			case envPairIndent < 0:
				envPairIndent = indent
			case indent != envPairIndent:
				return nil, fmt.Errorf("line %d: %q is indented %d spaces, the other variables are indented %d", n, key, indent, envPairIndent)
			}
			if key == "" {
				return nil, fmt.Errorf("line %d: environment variable name is empty", n)
			}
			if _, dup := current.Env[key]; dup {
				return nil, fmt.Errorf("line %d: %s %q is set twice", n, fieldEnv, key)
			}
			current.Env[key] = value
			continue
		}
		envIndent, envPairIndent = -1, -1

		switch {
		case fieldIndent < 0:
			fieldIndent = indent
		case indent != fieldIndent:
			return nil, fmt.Errorf("line %d: %q is indented %d spaces, the other fields are indented %d", n, key, indent, fieldIndent)
		}

		switch key {
		case fieldRun:
			if current.Run != "" {
				return nil, fmt.Errorf("line %d: %s is set twice", n, fieldRun)
			}
			if value == "" {
				return nil, fmt.Errorf("line %d: %s is empty", n, fieldRun)
			}
			current.Run = value
		case fieldDir:
			if current.Dir != "" {
				return nil, fmt.Errorf("line %d: %s is set twice", n, fieldDir)
			}
			if value == "" {
				return nil, fmt.Errorf("line %d: %s is empty", n, fieldDir)
			}
			current.Dir = value
		case fieldEnv:
			if current.Env != nil {
				return nil, fmt.Errorf("line %d: %s is set twice", n, fieldEnv)
			}
			if value != "" {
				return nil, fmt.Errorf("line %d: %s takes no value, put the variables on the lines below it", n, fieldEnv)
			}
			current.Env = map[string]string{}
			envIndent = indent
		default:
			return nil, fmt.Errorf("line %d: unknown field %q", n, key)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := closeCurrent(); err != nil {
		return nil, err
	}

	for _, entry := range entries {
		for i, r := range entry.Name {
			if r != '/' {
				continue
			}
			folder := entry.Name[:i]
			if at, ok := lines[folder]; ok {
				return nil, fmt.Errorf("line %d: %q is a folder here and a command on line %d", lines[entry.Name], folder, at)
			}
		}
	}
	return entries, nil
}

// checkName reports whether a command name is well formed. Spaces inside a
// folder are part of its name, so "my services/the api" is fine, but a space
// against a "/" is a typo rather than a second way to write the same name.
func checkName(name string) error {
	if name == "" {
		return errors.New("command name is empty")
	}
	for _, folder := range strings.Split(name, "/") {
		trimmed := strings.TrimSpace(folder)
		if trimmed == "" {
			return fmt.Errorf("%q has an empty folder", name)
		}
		if trimmed != folder {
			return fmt.Errorf("%q has spaces around %q", name, trimmed)
		}
	}
	return nil
}

// readRunbookfile parses the Runbookfile at path. Parse errors carry the file
// name in front of the line they come from.
func readRunbookfile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries, err := parseRunbookfile(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return entries, nil
}

// printEntries writes the commands back out, in the order they were read, so
// what Runbook understood can be checked against the file.
func printEntries(w io.Writer, entries []Entry) {
	for _, entry := range entries {
		fmt.Fprintf(w, "%s:\n", entry.Name)
		fmt.Fprintf(w, "  %s: %s\n", fieldRun, entry.Run)
		if entry.Dir != "" {
			fmt.Fprintf(w, "  %s: %s\n", fieldDir, entry.Dir)
		}
		if len(entry.Env) > 0 {
			fmt.Fprintf(w, "  %s:\n", fieldEnv)
			for _, name := range slices.Sorted(maps.Keys(entry.Env)) {
				fmt.Fprintf(w, "    %s: %s\n", name, entry.Env[name])
			}
		}
	}
}

// printNames writes one command name per line, in the order the Runbookfile
// lists them. It is what shell completion reads.
func printNames(w io.Writer, entries []Entry) {
	for _, entry := range entries {
		fmt.Fprintln(w, entry.Name)
	}
}
