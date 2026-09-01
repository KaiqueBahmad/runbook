package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
  run <name>   run one of them in this terminal, and exit with its status
  start <name> run one in the background, its output going to a log file
  stop <name>  end a command that was started
  status       show which commands are running, and at which process id
  completion   print a completion script for bash, zsh or fish

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
const (
	cmdList       = "list"
	cmdRun        = "run"
	cmdStart      = "start"
	cmdStop       = "stop"
	cmdStatus     = "status"
	cmdCompletion = "completion"
)

// commands is every command runbook answers to.
var commands = []string{cmdList, cmdRun, cmdStart, cmdStop, cmdStatus, cmdCompletion}

// named are the commands that take the name of a command in the Runbookfile.
var named = []string{cmdRun, cmdStart, cmdStop}

// invocation is what a command line asked for.
type invocation struct {
	cmd  string   // the command to carry out, empty when none was given
	rest []string // the arguments that command was given
	path string   // full path of the Runbookfile to work on
}

// parseArgs turns the command line arguments (without the program name) into
// the command to carry out and the full path of the Runbookfile to work on.
// The command is empty when none was given. The Runbookfile defaults to the
// one in the current directory, and any other path has to come after -f or
// --file; relative paths are resolved against the current directory. A --help
// or -h anywhere in the arguments returns errHelpRequested instead.
func parseArgs(args []string) (invocation, error) {
	var in invocation
	var path string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h":
			return invocation{}, errHelpRequested

		case arg == fileFlagShort || arg == fileFlag:
			if path != "" {
				return invocation{}, fmt.Errorf("%s is given twice", arg)
			}
			i++
			if i == len(args) {
				return invocation{}, fmt.Errorf("missing runbookfile path after %s", arg)
			}
			if args[i] == "" {
				return invocation{}, errors.New("runbookfile path is empty")
			}
			path = args[i]

		case strings.HasPrefix(arg, "-"):
			return invocation{}, fmt.Errorf("unknown flag %q", arg)

		case in.cmd != "":
			in.rest = append(in.rest, arg)

		case slices.Contains(commands, arg):
			in.cmd = arg

		default:
			return invocation{}, fmt.Errorf("unknown command %q", arg)
		}
	}

	if err := checkRest(in); err != nil {
		return invocation{}, err
	}

	if path == "" {
		path = defaultRunbookfile
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return invocation{}, fmt.Errorf("resolving %q: %w", path, err)
	}
	in.path = full
	return in, nil
}

// checkRest reports whether a command was given the arguments it takes.
func checkRest(in invocation) error {
	switch in.cmd {
	case cmdCompletion:
		switch {
		case len(in.rest) == 0:
			return fmt.Errorf("%s needs a shell: %s", cmdCompletion, strings.Join(shells, ", "))
		case !slices.Contains(shells, in.rest[0]):
			return fmt.Errorf("unknown shell %q, want %s", in.rest[0], strings.Join(shells, ", "))
		}
		in.rest = in.rest[1:]
	default:
		if slices.Contains(named, in.cmd) {
			if len(in.rest) == 0 {
				return fmt.Errorf("%s needs the name of a command", in.cmd)
			}
			in.rest = in.rest[1:]
		}
	}
	if len(in.rest) > 0 {
		return fmt.Errorf("unexpected argument %q", in.rest[0])
	}
	return nil
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
