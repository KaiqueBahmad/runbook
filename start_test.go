package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// startTest starts a command in a temporary directory and gives back that
// directory and its state file, with the command stopped again when the test
// ends. A command that has to say when it is ready can touch "ready" in the
// directory, which waitFor watches.
//
// The broadcaster it starts alongside is this very test binary, which TestMain
// sends to broadcast when it is asked for it.
func startTest(t *testing.T, run string) (state, string, string) {
	t.Helper()

	base := t.TempDir()
	stateFile := filepath.Join(base, ".runbook", "state", "api.pid")

	if _, err := startEntry(Entry{Name: "api", Run: run}, base, stateFile, testAddr(base)); err != nil {
		t.Fatalf("startEntry(): %v", err)
	}
	st, err := readState(stateFile)
	if err != nil {
		t.Fatalf("readState(): %v", err)
	}
	t.Cleanup(func() { syscall.Kill(-st.group(), syscall.SIGKILL) })

	return st, stateFile, base
}

func TestStartEntry(t *testing.T) {
	t.Run("records a process that is running", func(t *testing.T) {
		st, _, _ := startTest(t, "sleep 30")

		if !st.alive() {
			t.Error("the command is not running")
		}
		// The command leads a group of its own, which is what lets stop reach
		// everything it spawns. Ask the kernel rather than the state file.
		if group, err := syscall.Getpgid(st.PID); err != nil || group != st.PID {
			t.Errorf("the group of %d is %d (%v), want it to be its own leader", st.PID, group, err)
		}
		if st.Boot == "" {
			t.Error("no start time recorded, a reused pid would go unnoticed")
		}
	})

	t.Run("a command already running is left alone", func(t *testing.T) {
		st, stateFile, _ := startTest(t, "sleep 30")

		base := t.TempDir()
		_, err := startEntry(Entry{Name: "api", Run: "sleep 30"}, base, stateFile, testAddr(base))
		if err == nil {
			t.Fatal("startEntry() error = nil, want an error")
		}
		if !strings.Contains(err.Error(), "already running") {
			t.Errorf("startEntry() error = %v, want it to say the command is running", err)
		}
		if !st.alive() {
			t.Error("the first command was stopped")
		}
	})

	t.Run("a command that ended is started again", func(t *testing.T) {
		base := t.TempDir()
		stateFile := filepath.Join(base, "api.pid")

		if _, err := startEntry(Entry{Name: "api", Run: "true"}, base, stateFile, testAddr(base)); err != nil {
			t.Fatalf("startEntry(): %v", err)
		}
		st, _ := readState(stateFile)
		waitFor(t, func() bool { return !st.alive() })

		if _, err := startEntry(Entry{Name: "api", Run: "sleep 30"}, base, stateFile, testAddr(base)); err != nil {
			t.Errorf("startEntry() on a finished command: %v", err)
		}
		st, _ = readState(stateFile)
		t.Cleanup(func() { syscall.Kill(-st.group(), syscall.SIGKILL) })
	})
}

func TestStopEntry(t *testing.T) {
	t.Run("stops the whole process group", func(t *testing.T) {
		// The shell spawns a child of its own, which is the thing that
		// signalling only the shell would leave behind.
		st, stateFile, _ := startTest(t, "sleep 30 & wait")

		waitFor(t, func() bool { return len(groupOf(st.group())) > 1 })
		members := groupOf(st.group())
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
		if st.alive() {
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
		if st.alive() {
			t.Error("the command is still running")
		}
	})

	t.Run("a command that was never started", func(t *testing.T) {
		_, err := stopEntry(filepath.Join(t.TempDir(), "api.pid"), time.Second)
		if err == nil || !strings.Contains(err.Error(), "not running") {
			t.Errorf("stopEntry() error = %v, want it to say the command is not running", err)
		}
	})

	t.Run("a state file left behind by a process that is gone", func(t *testing.T) {
		stateFile := filepath.Join(t.TempDir(), "api.pid")
		if err := writeState(stateFile, stateOf(0x7FFFFFFF, "1")); err != nil {
			t.Fatalf("writeState(): %v", err)
		}

		_, err := stopEntry(stateFile, time.Second)
		if err == nil || !strings.Contains(err.Error(), "not running") {
			t.Errorf("stopEntry() error = %v, want it to say the command is not running", err)
		}
		if _, err := os.Stat(stateFile); err == nil {
			t.Error("the stale state file was left in place")
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

// waitFor gives a condition a couple of seconds to come true, for the moments
// where a command Runbook started has to get somewhere first.
func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	for range 100 {
		if done() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("waited two seconds and it never happened")
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}
