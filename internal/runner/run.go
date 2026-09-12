package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"runbook/internal/runbookfile"
)

// Run runs one of a runbook.yml's commands in this terminal, and returns the
// status it exited with, so Runbook can exit with the same one.
func Run(path string, entries []runbookfile.Entry, name string, stdout, stderr io.Writer) (int, error) {
	entry, err := runbookfile.Find(entries, name)
	if err != nil {
		return 0, err
	}
	return runEntry(entry, filepath.Dir(path), stdout, stderr)
}

// runEntry runs one command in the foreground and returns the status it exited
// with. A command killed by a signal reports 128 plus that signal, the way a
// shell does.
func runEntry(entry runbookfile.Entry, base string, stdout, stderr io.Writer) (int, error) {
	cmd := exec.Command(shell, shellFlag, entry.Run)
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

	code, err := exitStatus(cmd.Wait())
	if err != nil {
		return 0, fmt.Errorf("running %s: %w", entry.Name, err)
	}
	return code, nil
}
