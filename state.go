package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// stateDirExt names the directory Runbook keeps the state of one Runbookfile's
// commands in, and stateExt the file of one command inside it.
const (
	stateDirExt = ".state"
	stateExt    = ".pid"
)

// state is what Runbook remembers about a command it started, so that a later
// runbook stop, from another terminal, can find the process again. It is three
// numbers, and it is written as one line:
//
//	557439 11132527 1756699472
type state struct {
	PID   int    // the process Runbook started
	Boot  string // the time the kernel says it started, in ticks since boot
	Since int64  // the time Runbook started it, in seconds since the epoch
}

// group is the process group to signal. startEntry puts the command in a group
// of its own and it is the leader of it, so the group carries its number.
func (st state) group() int {
	return st.PID
}

// stateDir is where the state of one Runbookfile's commands lives. It sits
// inside .runbook, named after the Runbookfile, so two Runbookfiles side by
// side do not share it.
func stateDir(path string) string {
	return filepath.Join(filepath.Dir(path), runbookDirName, filepath.Base(path)+stateDirExt)
}

// stateFile is where one command's state lives. A command name is a path
// already, so its folders become directories.
func stateFile(path, name string) string {
	return filepath.Join(stateDir(path), filepath.FromSlash(name)+stateExt)
}

// readState reads back what start remembered. A command that was never started
// gives an error matching fs.ErrNotExist.
func readState(file string) (state, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return state{}, err
	}
	fields := strings.Fields(string(data))
	if len(fields) != 3 {
		return state{}, fmt.Errorf("%s holds %q, want a pid, a boot time and a start time", file, data)
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return state{}, fmt.Errorf("%s holds the pid %q: %w", file, fields[0], err)
	}
	since, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return state{}, fmt.Errorf("%s holds the start time %q: %w", file, fields[2], err)
	}
	return state{PID: pid, Boot: fields[1], Since: since}, nil
}

// writeState records a started command, creating the directories its name asks
// for.
func writeState(file string, st state) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(file), err)
	}
	line := fmt.Sprintf("%d %s %d\n", st.PID, st.Boot, st.Since)
	if err := os.WriteFile(file, []byte(line), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	return nil
}

// alive reports whether the recorded process is still running, and is still the
// same one. The kernel hands out process ids again once they are free, so the
// start time has to match too, or a stale file would have Runbook signal
// whatever inherited the number.
func (st state) alive() bool {
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

// processBoot is the time the kernel says a process started, in clock ticks
// since the machine booted.
func processBoot(pid int) (string, error) {
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

// since is how long ago a command was started, for a message.
func (st state) since() time.Duration {
	if st.Since == 0 {
		return 0
	}
	return time.Since(time.Unix(st.Since, 0)).Truncate(time.Second)
}

// sweep forgets the commands of a Runbookfile that have ended since they were
// started: the state files whose process is gone, and the folders left empty
// once those are removed. A Runbookfile that never started anything sweeps
// clean without complaint.
//
// It is housekeeping, not correctness: every command already treats a missing
// state file and a dead process the same way, and an address with nobody
// behind it is bound over on the next start. What it catches that they do not
// is what a command has left behind since it was renamed or taken out of the
// Runbookfile, which nothing else ever looks at again.
func sweep(path string) error {
	if err := sweepUnder(stateDir(path), deadState); err != nil {
		return err
	}
	// The addresses the broadcasters listen at are swept the same way, and are
	// past when there is nobody behind them any more.
	return sweepIPC(path)
}

// sweepUnder clears the files under dir that gone reports are past, and the
// folders left empty once they are, dir itself included. A directory that was
// never there sweeps clean without complaint.
func sweepUnder(dir string, gone func(file string) bool) error {
	empty, err := sweepDir(dir, gone)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if empty {
		return remove(dir)
	}
	return nil
}

// deadState reports whether a state file is one whose process has gone.
func deadState(file string) bool {
	if !strings.HasSuffix(file, stateExt) {
		return false
	}
	// A file this Runbook cannot read stays. It may have been written by a
	// newer one that records more, and removing it would leave the process it
	// describes running with nobody left who knows its number.
	st, err := readState(file)
	return err == nil && !st.alive()
}

// sweepDir clears what gone reports is past under dir, and reports whether
// anything is left in it afterwards.
func sweepDir(dir string, gone func(file string) bool) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	kept := 0
	for _, entry := range entries {
		name := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			empty, err := sweepDir(name, gone)
			if err != nil {
				return false, err
			}
			if !empty {
				kept++
				continue
			}
			if err := remove(name); err != nil {
				return false, err
			}
			continue
		}

		if !gone(name) {
			kept++
			continue
		}
		if err := remove(name); err != nil {
			return false, err
		}
	}
	return kept == 0, nil
}

// remove deletes a file or an empty directory, and is happy if another Runbook
// got there first.
func remove(name string) error {
	if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
