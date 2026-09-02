// Package cli is the command line: what the arguments ask for, the help and
// the completion scripts, and carrying the answer out.
package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"runbook/internal/runner"
)

// defaultFile is the file Runbook looks for when no path is given.
const defaultFile = "runbook.yml"

const usage = "usage: runbook [-f runbook.yml] [command]"

// helpHint points at the help text. It is printed instead of the usage line
// when the arguments are wrong.
const helpHint = "run 'runbook --help' for usage"

// help is the full text printed for --help and -h.
const help = usage + `

Runbook opens a GUI control panel for the commands listed in a runbook.yml.

With no arguments it looks for a runbook.yml in the current directory; pass a
path after -f to open a different file instead.

Commands:
  list         print the name of every command in the runbook.yml
  run <name>   run one of them in this terminal, and exit with its status
  start <name> run one in the background, where its output is broadcast
  stop <name>  end a command that was started
  status       show which commands are running, and at which process id
  logs <name>  listen to what a started command writes, from now on
  completion   print a completion script for bash, zsh or fish

Options:
  -f, --file   path of the runbook.yml to open
  -h, --help   print this help and exit`

// errHelpRequested is returned by parseArgs when the arguments ask for the
// help text. It is not a failure: main prints help and exits successfully.
var errHelpRequested = errors.New("help requested")

// fileFlag and fileFlagShort introduce the path of the runbook.yml to open.
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
	cmdLogs       = "logs"
	cmdCompletion = "completion"
)

// cmdBroadcast is Runbook talking to itself, one broadcaster per started
// command. It is left out of the list below, of the help and of the completion
// scripts: it takes an address rather than the name of a command, and there is
// nothing there for a person to do.
const cmdBroadcast = runner.BroadcastCommand

// commands is every command runbook offers.
var commands = []string{cmdList, cmdRun, cmdStart, cmdStop, cmdStatus, cmdLogs, cmdCompletion}

// named are the commands that take the name of a command in the runbook.yml.
var named = []string{cmdRun, cmdStart, cmdStop, cmdLogs}

// invocation is what a command line asked for.
type invocation struct {
	cmd  string   // the command to carry out, empty when none was given
	rest []string // the arguments that command was given
	path string   // full path of the runbook.yml to work on
}

// parseArgs turns the command line arguments (without the program name) into
// the command to carry out and the full path of the runbook.yml to work on.
// The command is empty when none was given. The runbook.yml defaults to the
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
				return invocation{}, fmt.Errorf("missing runbook.yml path after %s", arg)
			}
			if args[i] == "" {
				return invocation{}, errors.New("runbook.yml path is empty")
			}
			path = args[i]

		case strings.HasPrefix(arg, "-"):
			return invocation{}, fmt.Errorf("unknown flag %q", arg)

		case in.cmd != "":
			in.rest = append(in.rest, arg)

		case slices.Contains(commands, arg) || arg == cmdBroadcast:
			in.cmd = arg

		default:
			return invocation{}, fmt.Errorf("unknown command %q", arg)
		}
	}

	if err := checkRest(in); err != nil {
		return invocation{}, err
	}

	if path == "" {
		path = defaultFile
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
	case cmdBroadcast:
		if len(in.rest) == 0 {
			return fmt.Errorf("%s needs an address to listen on", cmdBroadcast)
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
