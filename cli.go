package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// defaultRunbookfile is the file Runbook looks for when no path is given.
const defaultRunbookfile = "Runbookfile"

const usage = "usage: runbook [-f runbookfile] [command]"

// helpHint points at the help text. It is printed instead of the usage line
// when the arguments are wrong.
const helpHint = "run 'runbook --help' for usage"

// help is the full text printed for --help and -h.
const help = usage + `

Runbook opens a GUI control panel for the commands listed in a Runbookfile.

With no arguments it looks for a Runbookfile in the current directory; pass a
path after -f to open a different file instead.

Commands:
  list         print the name of every command in the Runbookfile

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

// The commands runbook takes. An empty command opens the panel.
const cmdList = "list"

// parseArgs turns the command line arguments (without the program name) into
// the command to carry out and the full path of the Runbookfile to work on.
// The command is empty when none was given. The Runbookfile defaults to the
// one in the current directory, and any other path has to come after -f or
// --file; relative paths are resolved against the current directory. A --help
// or -h anywhere in the arguments returns errHelpRequested instead.
func parseArgs(args []string) (string, string, error) {
	var cmd, path string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h":
			return "", "", errHelpRequested

		case arg == fileFlagShort || arg == fileFlag:
			if path != "" {
				return "", "", fmt.Errorf("%s is given twice", arg)
			}
			i++
			if i == len(args) {
				return "", "", fmt.Errorf("missing runbookfile path after %s", arg)
			}
			if args[i] == "" {
				return "", "", errors.New("runbookfile path is empty")
			}
			path = args[i]

		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("unknown flag %q", arg)

		case cmd != "":
			return "", "", fmt.Errorf("unexpected argument %q", arg)

		case arg == cmdList:
			cmd = arg

		default:
			return "", "", fmt.Errorf("unknown command %q", arg)
		}
	}

	if path == "" {
		path = defaultRunbookfile
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolving %q: %w", path, err)
	}
	return cmd, full, nil
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
