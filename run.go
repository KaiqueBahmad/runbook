package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
)

// shell is what every command in a Runbookfile is handed to, so a command
// behaves the same whatever shell the person running it uses.
const shell = "sh"

// findEntry looks up a command by the name a Runbookfile gave it.
func findEntry(entries []Entry, name string) (Entry, error) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, nil
		}
	}
	return Entry{}, fmt.Errorf("no command named %q, run 'runbook list' to see them", name)
}

// entryDir is the directory a command runs in. base is the directory the
// Runbookfile lives in, which a relative dir is measured from.
func entryDir(entry Entry, base string) string {
	switch {
	case entry.Dir == "":
		return base
	case filepath.IsAbs(entry.Dir):
		return entry.Dir
	default:
		return filepath.Join(base, entry.Dir)
	}
}

// entryEnv is the environment a command runs with: the one Runbook was started
// with, plus the command's own variables. It is nil when the command adds
// none, which leaves the environment untouched.
func entryEnv(entry Entry) []string {
	if len(entry.Env) == 0 {
		return nil
	}
	env := os.Environ()
	for _, name := range slices.Sorted(maps.Keys(entry.Env)) {
		env = append(env, name+"="+entry.Env[name])
	}
	return env
}

// runEntry runs one command in the foreground and returns the status it exited
// with, so Runbook can exit with the same one. A command killed by a signal
// reports 128 plus that signal, the way a shell does.
func runEntry(entry Entry, base string, stdout, stderr io.Writer) (int, error) {
	cmd := exec.Command(shell, "-c", entry.Run)
	cmd.Dir = entryDir(entry, base)
	cmd.Env = entryEnv(entry)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("starting %s: %w", entry.Name, err)
	}

	// Ctrl-C reaches the whole foreground group, so the command gets it
	// straight from the terminal. Runbook swallows its own copy to stay alive
	// long enough to report how the command ended.
	defer ignoreInterrupts()()

	err := cmd.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exit):
		if code := exit.ExitCode(); code >= 0 {
			return code, nil
		}
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal()), nil
		}
		return 1, nil
	default:
		return 0, fmt.Errorf("running %s: %w", entry.Name, err)
	}
}

// ignoreInterrupts stops the interrupt signals from ending Runbook itself, and
// returns the function that puts them back.
func ignoreInterrupts() func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range signals {
		}
	}()
	return func() {
		signal.Stop(signals)
		close(signals)
	}
}
