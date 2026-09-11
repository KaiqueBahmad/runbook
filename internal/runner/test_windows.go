//go:build windows

package runner

import (
	"testing"
)

// groupOf is the pids that belong to a process group. On Windows, process
// groups are not easily enumerable, so we return an empty list. Tests that
// depend on this will be skipped.
func groupOf(pgid int) []int {
	return nil
}

// killTestProcess kills a process for test cleanup. On Windows, we terminate
// the process directly since there is no process group signaling.
func killTestProcess(t *testing.T, pid int) {
	t.Helper()
	killGroup(pid)
}
