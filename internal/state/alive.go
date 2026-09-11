//go:build !windows

package state

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
)

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
