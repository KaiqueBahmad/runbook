//go:build !windows

package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestStartEntryGroup is the started command leading a process group of its
// own, which is what lets stop reach everything it spawns. It asks the kernel
// rather than the state file. Windows has no process groups to ask about.
func TestStartEntryGroup(t *testing.T) {
	st, _, _ := startTest(t, cmdSleep)

	if group, err := syscall.Getpgid(st.PID); err != nil || group != st.PID {
		t.Errorf("the group of %d is %d (%v), want it to be its own leader", st.PID, group, err)
	}
}

// TestStopEntrySignals is what stopping means where there are signals: the
// whole group goes, and a command that ignores the request is killed.
func TestStopEntrySignals(t *testing.T) {
	t.Run("stops the whole process group", func(t *testing.T) {
		// The shell spawns a child of its own, which is the thing that
		// signalling only the shell would leave behind.
		st, stateFile, _ := startTest(t, "sleep 30 & wait")

		waitFor(t, func() bool { return len(groupOf(st.Group())) > 1 })
		members := groupOf(st.Group())
		if len(members) < 2 {
			t.Skip("the shell never spawned a child to look at")
		}
		child := members[len(members)-1]

		killed, err := stopEntry(stateFile, time.Second)
		if err != nil {
			t.Fatalf("stopEntry(): %v", err)
		}
		if killed {
			t.Error("stopEntry() had to kill a command that asks nothing of a signal")
		}
		if st.Alive() {
			t.Error("the command is still running")
		}
		if syscall.Kill(child, 0) == nil {
			t.Errorf("the child %d outlived its group", child)
		}
		if _, err := os.Stat(stateFile); err == nil {
			t.Error("the state file is still there")
		}
	})

	t.Run("kills a command that ignores the request", func(t *testing.T) {
		// The shell says so once the trap is in place: stopping it before that
		// would take the default action and prove nothing.
		st, stateFile, base := startTest(t, "trap '' TERM; touch ready; while true; do sleep 1; done")
		waitFor(t, func() bool { return exists(filepath.Join(base, "ready")) })

		killed, err := stopEntry(stateFile, 300*time.Millisecond)
		if err != nil {
			t.Fatalf("stopEntry(): %v", err)
		}
		if !killed {
			t.Error("stopEntry() reports a clean stop, want it to report the kill")
		}
		if st.Alive() {
			t.Error("the command is still running")
		}
	})
}

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
