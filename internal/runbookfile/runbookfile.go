// Package runbookfile reads the runbook.yml: the file a project lists its
// commands in, and the entries that come out of it.
package runbookfile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Entry is one command from the runbook.yml.
type Entry struct {
	Name        string            // slashes group entries into folders, e.g. "services/api"
	Run         string            // shell command
	Description string            // optional one line summary of what the command does
	Dir         string            // optional working directory, relative to the runbook.yml's directory
	Env         map[string]string // optional extra environment variables, nil if none
}

// Name is what the file is called. Runbook looks for it by this name, and
// nothing else, when it is not given a path of its own.
const Name = "runbook.yml"

const (
	fieldRun         = "run"
	fieldDescription = "description"
	fieldDir         = "dir"
	fieldEnv         = "env"
)

// Parse reads a runbook.yml and returns the commands it lists. A command is a
// name, and under it the fields it is made of:
//
//	name:                the name of the command, slashes group it into folders
//	  run:               the shell command to run
//	  description:       optional one line summary of what it does
//	  dir:               optional working directory, relative to the
//	                     runbook.yml's own, and defaulting to it
//	  env:               optional extra environment variables
//	    KEY: value
//
// For example:
//
//	services/api:
//	  run: go run main.go
//	  dir: ./api
//	  env:
//	    PORT: 8080
//	    DATABASE_URL: postgres://user:password@localhost:5432/db
//
// A run that does not fit on one line is written as a "|", with the lines of
// the command indented under it. They are handed to the shell as they are
// written, comments and blank lines and all:
//
//	db/reset:
//	  run: |
//	    dropdb app
//	    createdb app
//	    psql app < schema.sql
func Parse(r io.Reader) ([]Entry, error) {
	var (
		entries []Entry
		lines   = map[string]int{} // command name -> line it was opened on

		current  *Entry
		headerAt int

		fieldIndent   = -1
		envIndent     = -1
		envPairIndent = -1

		blockAt     = -1 // line the "run: |" was opened on, -1 when not in a block
		blockIndent = -1 // indent the block's lines are written at, -1 until its first
		blockLines  []string
	)

	// closeBlock makes what a "run: |" collected the command. The blank lines
	// that trail it go: a shell command is the same without them, and an empty
	// block is then plainly empty.
	closeBlock := func() error {
		for len(blockLines) > 0 && blockLines[len(blockLines)-1] == "" {
			blockLines = blockLines[:len(blockLines)-1]
		}
		if len(blockLines) == 0 {
			return fmt.Errorf("line %d: %s is empty", blockAt, fieldRun)
		}
		current.Run = strings.Join(blockLines, "\n")
		blockAt, blockIndent, blockLines = -1, -1, nil
		return nil
	}

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

		// Inside a block every line belongs to the command until one comes
		// back out to the fields, so a "#" here is the shell's comment rather
		// than the file's, and a blank line is a line of the command.
		if blockAt >= 0 {
			spaces := leadingSpaces(line)
			switch {
			case line == "":
				blockLines = append(blockLines, "")
				continue
			case line[spaces] == '\t' && (blockIndent < 0 || spaces < blockIndent):
				return nil, fmt.Errorf("line %d: indent with spaces, not tabs", n)
			case blockIndent < 0 && spaces > fieldIndent:
				blockIndent = spaces // the first line sets where the block sits
				fallthrough
			case blockIndent >= 0 && spaces >= blockIndent:
				blockLines = append(blockLines, line[blockIndent:])
				continue
			}
			// The line is back out at the fields: the block ended above it,
			// and the line itself is read as one of them.
			if err := closeBlock(); err != nil {
				return nil, err
			}
		}

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
			if blockHeader(value) {
				return nil, blockNotAllowed(n, fmt.Sprintf("%s %q", fieldEnv, key))
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
			if blockHeader(value) {
				blockAt = n // the command is on the lines below, indented under this one
				continue
			}
			if strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">") {
				return nil, fmt.Errorf("line %d: %q is not a command; write %q on its own to put a multiline one on the lines below it", n, value, "|")
			}
			current.Run = value
		case fieldDescription:
			if current.Description != "" {
				return nil, fmt.Errorf("line %d: %s is set twice", n, fieldDescription)
			}
			if value == "" {
				return nil, fmt.Errorf("line %d: %s is empty", n, fieldDescription)
			}
			if blockHeader(value) {
				return nil, blockNotAllowed(n, fieldDescription)
			}
			current.Description = value
		case fieldDir:
			if current.Dir != "" {
				return nil, fmt.Errorf("line %d: %s is set twice", n, fieldDir)
			}
			if value == "" {
				return nil, fmt.Errorf("line %d: %s is empty", n, fieldDir)
			}
			if blockHeader(value) {
				return nil, blockNotAllowed(n, fieldDir)
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
	if blockAt >= 0 {
		if err := closeBlock(); err != nil {
			return nil, err
		}
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

// blockHeader reports whether a field's value opens a "|" block: the line
// holds the indicator alone, and the value is on the lines below it. A shell
// command never reads this way, since neither "|" nor ">" can open one, so the
// two never stand for the same thing.
//
// YAML also writes the chomping indicators "|-" and "|+" here, which say what
// to do with the newlines that trail the block. A command runs the same either
// way, so they are all read as a plain "|".
func blockHeader(value string) bool {
	switch value {
	case "|", "|-", "|+":
		return true
	}
	return false
}

// blockNotAllowed says that a field other than the run command was given a
// block, which only the command itself can be.
func blockNotAllowed(n int, what string) error {
	return fmt.Errorf("line %d: %s takes a single line, only %s can be a %q block", n, what, fieldRun, "|")
}

// leadingSpaces is how many spaces a line begins with. It counts spaces alone,
// where trimming would take the tabs a block's lines are free to start with.
func leadingSpaces(line string) int {
	n := 0
	for n < len(line) && line[n] == ' ' {
		n++
	}
	return n
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
		// A name becomes a path under .runbook, so it may not climb out of it.
		if trimmed == "." || trimmed == ".." {
			return fmt.Errorf("%q has a folder named %q", name, trimmed)
		}
		if trimmed != folder {
			return fmt.Errorf("%q has spaces around %q", name, trimmed)
		}
	}
	return nil
}

// Find looks up a command by the name a runbook.yml gave it.
func Find(entries []Entry, name string) (Entry, error) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, nil
		}
	}
	return Entry{}, fmt.Errorf("no command named %q, run 'runbook list' to see them", name)
}

// Check reports whether path is a readable regular file.
func Check(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("no runbook.yml at %s", path)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a runbook.yml", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}

// Locate finds the runbook.yml a directory belongs to: the one in dir, or
// failing that the one in the nearest directory above it that has one, up to
// the root. It is what Runbook looks for when it is given no path, so that a
// project answers for its commands from anywhere inside it, and not only from
// the one directory its runbook.yml happens to sit in.
//
// A directory or anything else wearing the name is not the file, so the walk
// steps over it and keeps climbing.
func Locate(dir string) (string, error) {
	for up := dir; ; {
		path := filepath.Join(up, Name)
		if Check(path) == nil {
			return path, nil
		}
		parent := filepath.Dir(up)
		if parent == up {
			return "", fmt.Errorf("no %s in %s or any directory above it", Name, dir)
		}
		up = parent
	}
}

// Read opens the runbook.yml at path and parses it.
func Read(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return entries, nil
}

// PrintNames writes one command per line, in the order the runbook.yml lists
// them: the name, then its description. A command with no description is the
// name on its own.
//
// Aligned, the descriptions line up in a column for someone reading them. Not
// aligned, a tab separates the two, which is what shell completion reads: the
// separator zsh and fish show a description behind, and that bash cuts away.
func PrintNames(w io.Writer, entries []Entry, align bool) {
	width := 0
	if align {
		for _, entry := range entries {
			if entry.Description == "" {
				continue
			}
			width = max(width, utf8.RuneCountInString(entry.Name))
		}
	}

	for _, entry := range entries {
		if entry.Description == "" {
			fmt.Fprintln(w, entry.Name)
			continue
		}
		if !align {
			fmt.Fprintf(w, "%s\t%s\n", entry.Name, entry.Description)
			continue
		}
		pad := strings.Repeat(" ", width-utf8.RuneCountInString(entry.Name))
		fmt.Fprintf(w, "%s%s  %s\n", entry.Name, pad, entry.Description)
	}
}
