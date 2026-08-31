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

// help is the full text printed for --help and -h.
const help = usage + `

Runbook opens a GUI control panel for the commands listed in a Runbookfile.

With no arguments it looks for a Runbookfile in the current directory; pass a
path to open a different file instead.

Options:
  -h, --help   print this help and exit`

// errHelpRequested is returned by parseArgs when the arguments ask for the
// help text. It is not a failure: main prints help and exits successfully.
var errHelpRequested = errors.New("help requested")

// parseArgs turns the command line arguments (without the program name) into
// the full path of the Runbookfile to open. It accepts nothing or a single
// path; relative paths are resolved against the current directory. A --help or
// -h anywhere in the arguments returns errHelpRequested instead.
func parseArgs(args []string) (string, error) {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return "", errHelpRequested
		}
	}

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

// runbookDirName is the directory Runbook creates alongside the Runbookfile to
// keep its own files.
const runbookDirName = ".runbook"

// ensureRunbookDir returns the path of Runbook's directory next to the
// Runbookfile at path, creating it if it does not exist yet.
func ensureRunbookDir(path string) (string, error) {
	dir := filepath.Join(filepath.Dir(path), runbookDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	return dir, nil
}
