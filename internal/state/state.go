// Package state is what Runbook remembers about the commands it has started:
// one small file per running command, and the checks that say whether the
// process it names is still there.
package state

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"runbook/internal/workdir"
)

// dirExt names the directory Runbook keeps the state of one Runbookfile's
// commands in, and fileExt the file of one command inside it.
const (
	dirExt  = ".state"
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

// Dir is where the state of one Runbookfile's commands lives. It sits
// inside .runbook, named after the Runbookfile, so two Runbookfiles side by
// side do not share it.
func Dir(path string) string {
	return filepath.Join(filepath.Dir(path), workdir.Name, filepath.Base(path)+dirExt)
}

// File is where one command's state lives. A command name is a path
// already, so its folders become directories.
func File(path, name string) string {
	return filepath.Join(Dir(path), filepath.FromSlash(name)+fileExt)
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

// Alive reports whether the recorded process is still running, and is still the
// same one. The kernel hands out process ids again once they are free, so the
// start time has to match too, or a stale file would have Runbook signal
// whatever inherited the number.
func (st State) Alive() bool {
	err := syscall.Kill(st.PID, 0)
	if err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	letter, boot, err := processState(st.PID)
	if err != nil {
		// Without /proc the signal above is all there is to go on.
		return true
	}
	// A command that has ended but has not been collected by whoever started
	// it keeps its number and still answers a signal. It is not running.
	if letter == "Z" {
		return false
	}
	return st.Boot == "" || boot == st.Boot
}

// ProcessBoot is the time the kernel says a process started, in clock ticks
// since the machine booted.
func ProcessBoot(pid int) (string, error) {
	_, boot, err := processState(pid)
	return boot, err
}

// processState is what the kernel says about a process: the letter of its state
// and the time it started. It fails where there is no /proc, which only costs
// the checks above.
func processState(pid int) (string, string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", "", err
	}
	// The second field is the program name in parentheses and can hold spaces
	// and parentheses of its own, so the fields are counted from the last one.
	end := bytes.LastIndexByte(data, ')')
	if end < 0 {
		return "", "", fmt.Errorf("stat of %d has no program name", pid)
	}
	fields := strings.Fields(string(data[end+1:]))

	// The state is field 3 of the file, the first one after the program name,
	// and the start time is field 22.
	const bootField = 19
	if len(fields) <= bootField {
		return "", "", fmt.Errorf("stat of %d has %d fields", pid, len(fields))
	}
	return fields[0], fields[bootField], nil
}

// Uptime is how long ago a command was started, for a message.
func (st State) Uptime() time.Duration {
	if st.Since == 0 {
		return 0
	}
	return time.Since(time.Unix(st.Since, 0)).Truncate(time.Second)
}

// Sweep forgets the commands of a Runbookfile that have ended since they were
// started: the state files whose process is gone, and the folders left empty
// once those are removed. A Runbookfile that never started anything sweeps
// clean without complaint.
//
// It is housekeeping, not correctness: every command already treats a missing
// state file and a dead process the same way. What it catches that they do not
// is what a command has left behind since it was renamed or taken out of the
// Runbookfile, which nothing else ever looks at again.
func Sweep(path string) error {
	return workdir.SweepUnder(Dir(path), dead)
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
