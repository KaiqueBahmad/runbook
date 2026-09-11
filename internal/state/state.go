// Package state is what Runbook remembers about the commands it has started:
// one small file per running command, and the checks that say whether the
// process it names is still there.
package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"runbook/internal/workdir"
)

// dirName names the directory Runbook keeps the state of one runbook.yml's
// commands in, and fileExt the file of one command inside it.
const (
	dirName = "state"
	fileExt = ".pid"
)

// State is what Runbook remembers about a command it started, so that a later
// runbook stop, from another terminal, can find the process again. It is three
// numbers, and it is written as one line:
//
//	557439 11132527 1756699472
type State struct {
	PID   int    // the process Runbook started
	Boot  string // the time the kernel says it started, in ticks since boot
	Since int64  // the time Runbook started it, in seconds since the epoch
}

// Group is the process group to signal. startEntry puts the command in a group
// of its own and it is the leader of it, so the group carries its number.
func (st State) Group() int {
	return st.PID
}

// Dir is where the state of one runbook.yml's commands lives, inside the
// directory Runbook keeps for that file.
func Dir(work string) string {
	return filepath.Join(work, dirName)
}

// File is where one command's state lives. A command name is a path
// already, so its folders become directories.
func File(work, name string) string {
	return filepath.Join(Dir(work), filepath.FromSlash(name)+fileExt)
}

// Read reads back what start remembered. A command that was never started
// gives an error matching fs.ErrNotExist.
func Read(file string) (State, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return State{}, err
	}
	fields := strings.Fields(string(data))
	if len(fields) != 3 {
		return State{}, fmt.Errorf("%s holds %q, want a pid, a boot time and a start time", file, data)
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return State{}, fmt.Errorf("%s holds the pid %q: %w", file, fields[0], err)
	}
	since, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return State{}, fmt.Errorf("%s holds the start time %q: %w", file, fields[2], err)
	}
	return State{PID: pid, Boot: fields[1], Since: since}, nil
}

// Write records a started command, creating the directories its name asks
// for.
func Write(file string, st State) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(file), err)
	}
	line := fmt.Sprintf("%d %s %d\n", st.PID, st.Boot, st.Since)
	if err := os.WriteFile(file, []byte(line), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	return nil
}

// Uptime is how long ago a command was started, for a message.
func (st State) Uptime() time.Duration {
	if st.Since == 0 {
		return 0
	}
	return time.Since(time.Unix(st.Since, 0)).Truncate(time.Second)
}

// Sweep forgets the commands of a runbook.yml that have ended since they were
// started: the state files whose process is gone, and the folders left empty
// once those are removed. A runbook.yml that never started anything sweeps
// clean without complaint.
//
// It is housekeeping, not correctness: every command already treats a missing
// state file and a dead process the same way. What it catches that they do not
// is what a command has left behind since it was renamed or taken out of the
// runbook.yml, which nothing else ever looks at again.
func Sweep(work string) error {
	return workdir.SweepUnder(Dir(work), dead)
}

// dead reports whether a state file is one whose process has gone.
func dead(file string) bool {
	if !strings.HasSuffix(file, fileExt) {
		return false
	}
	// A file this Runbook cannot read stays. It may have been written by a
	// newer one that records more, and removing it would leave the process it
	// describes running with nobody left who knows its number.
	st, err := Read(file)
	return err == nil && !st.Alive()
}
