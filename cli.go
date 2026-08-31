package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// defaultRunbookfile is the file Runbook looks for when no path is given.
const defaultRunbookfile = "Runbookfile"

const usage = "usage: runbook [runbookfile]"

// parseArgs turns the command line arguments (without the program name) into
// the full path of the Runbookfile to open. It accepts nothing or a single
// path; relative paths are resolved against the current directory.
func parseArgs(args []string) (string, error) {
	var path string

	switch len(args) {
	case 0:
		path = defaultRunbookfile
	case 1:
		if args[0] == "" {
			return "", errors.New("runbookfile path is empty")
		}
		path = args[0]
	default:
		return "", fmt.Errorf("expected at most one runbookfile path, got %d arguments", len(args))
	}

	full, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", path, err)
	}
	return full, nil
}

// checkRunbookfile reports whether path is a readable regular file.
func checkRunbookfile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("no runbookfile at %s", path)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a runbookfile", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}
