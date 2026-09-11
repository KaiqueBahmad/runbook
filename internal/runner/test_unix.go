//go:build !windows

package runner

import (
	"os"
	"strconv"
	"syscall"
	"testing"
)

// groupOf is the pids that belong to a process group.
func groupOf(pgid int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var members []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if group, err := syscall.Getpgid(pid); err == nil && group == pgid {
			members = append(members, pid)
		}
	}
	return members
}

// killTestProcess kills a process group for test cleanup.
func killTestProcess(t *testing.T, pid int) {
	t.Helper()
	killGroup(pid)
}
