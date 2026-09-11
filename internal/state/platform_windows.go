//go:build windows

package state

import (
	"runbook/internal/winapi"
)

// Alive reports whether the recorded process is still running, and is still the
// same one. Windows hands out process ids again once they are free, so the
// start time has to match too, or a stale file would have Runbook take down
// whatever inherited the number.
func (st State) Alive() bool {
	if !winapi.Alive(st.PID) {
		return false
	}
	boot, err := winapi.CreationTime(st.PID)
	if err != nil {
		// Without a start time to compare, the check above is all there is.
		return true
	}
	return st.Boot == "" || boot == st.Boot
}

// ProcessBoot is when the kernel says a process started. It is only ever
// compared with another of its own kind, so what the number counts does not
// matter beyond being the same on both sides.
func ProcessBoot(pid int) (string, error) {
	return winapi.CreationTime(pid)
}
