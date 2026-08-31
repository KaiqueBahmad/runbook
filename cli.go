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

const usage = "usage: runbook [-f runbookfile]"

// helpHint points at the help text. It is printed instead of the usage line
// when the arguments are wrong.
const helpHint = "run 'runbook --help' for usage"

// help is the full text printed for --help and -h.
const help = usage + `

Runbook opens a GUI control panel for the commands listed in a Runbookfile.

With no arguments it looks for a Runbookfile in the current directory; pass a
path after -f to open a different file instead.

Options:
  -f, --file   path of the Runbookfile to open
  -h, --help   print this help and exit`

// errHelpRequested is returned by parseArgs when the arguments ask for the
// help text. It is not a failure: main prints help and exits successfully.
var errHelpRequested = errors.New("help requested")

// fileFlag and fileFlagShort introduce the path of the Runbookfile to open.
const (
	fileFlag      = "--file"
	fileFlagShort = "-f"
)

// parseArgs turns the command line arguments (without the program name) into
// the full path of the Runbookfile to open. With no arguments it falls back to
// the default Runbookfile; any other path has to come after -f or --file.
// Relative paths are resolved against the current directory. A --help or -h
// anywhere in the arguments returns errHelpRequested instead.
func parseArgs(args []string) (string, error) {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return "", errHelpRequested
		}
	}

	path := defaultRunbookfile

	switch len(args) {
	case 0:
	case 1, 2:
		if args[0] != fileFlagShort && args[0] != fileFlag {
			return "", fmt.Errorf("unexpected argument %q", args[0])
		}
		if len(args) == 1 {
			return "", fmt.Errorf("missing runbookfile path after %s", args[0])
		}
		if args[1] == "" {
			return "", errors.New("runbookfile path is empty")
		}
		path = args[1]
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

// gitignoreAll is the content Runbook puts in the directory's .gitignore. The
// pattern covers the ignore file itself, so nothing in there ever reaches the
// project's git status.
const gitignoreAll = "*\n"

// ensureRunbookDir returns the path of Runbook's directory next to the
// Runbookfile at path, creating it if it does not exist yet along with the
// .gitignore that keeps its contents out of the project's repository.
func ensureRunbookDir(path string) (string, error) {
	dir := filepath.Join(filepath.Dir(path), runbookDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}

	ignore := filepath.Join(dir, ".gitignore")
	switch _, err := os.Stat(ignore); {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.WriteFile(ignore, []byte(gitignoreAll), 0o600); err != nil {
			return "", fmt.Errorf("creating %s: %w", ignore, err)
		}
	case err != nil:
		return "", err
	}
	return dir, nil
}

// metadataExt is appended to the Runbookfile's name for the file where Runbook
// keeps what it knows about that Runbookfile.
const metadataExt = ".metadata"

// ensureMetadataFile returns the path of the metadata file inside dir for the
// Runbookfile at path, creating it empty if it does not exist yet.
func ensureMetadataFile(dir, path string) (string, error) {
	metadata := filepath.Join(dir, filepath.Base(path)+metadataExt)

	f, err := os.OpenFile(metadata, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", metadata, err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("closing %s: %w", metadata, err)
	}
	return metadata, nil
}
