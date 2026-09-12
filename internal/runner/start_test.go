package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"runbook/internal/ipc"
	"runbook/internal/runbookfile"
	"runbook/internal/state"
)

// testAddr is an address to broadcast on, kept short: a unix socket address is
// a path, and the kernel takes about a hundred characters of it.
func testAddr(base string) string {
	return filepath.Join(base, "api"+testAddrExt)
}

// startTest starts a command in a temporary directory and gives back that
// directory and its state file, with the command stopped again when the test
// ends. A command that has to say when it is ready can touch "ready" in the
// directory, which waitFor watches.
//
// The broadcaster it starts alongside is this very test binary, which TestMain
// sends to broadcast when it is asked for it.
func startTest(t *testing.T, run string) (state.State, string, string) {
	t.Helper()

	base := t.TempDir()
	stateFile := filepath.Join(base, ".runbook", "state", "api.pid")

	if _, err := startEntry(runbookfile.Entry{Name: "api", Run: run}, base, stateFile, testAddr(base)); err != nil {
		t.Fatalf("startEntry(): %v", err)
	}
	st, err := state.Read(stateFile)
	if err != nil {
		t.Fatalf("state.Read(): %v", err)
	}
	t.Cleanup(func() { killGroup(st.Group()) })

	return st, stateFile, base
}

func TestStartEntry(t *testing.T) {
	t.Run("records a process that is running", func(t *testing.T) {
		st, _, _ := startTest(t, cmdSleep)

		if !st.Alive() {
			t.Error("the command is not running")
		}
		if st.Boot == "" {
			t.Error("no start time recorded, a reused pid would go unnoticed")
		}
	})

	t.Run("a command already running is left alone", func(t *testing.T) {
		st, stateFile, _ := startTest(t, cmdSleep)

		base := t.TempDir()
		_, err := startEntry(runbookfile.Entry{Name: "api", Run: cmdSleep}, base, stateFile, testAddr(base))
		if err == nil {
			t.Fatal("startEntry() error = nil, want an error")
		}
		if !strings.Contains(err.Error(), "already running") {
			t.Errorf("startEntry() error = %v, want it to say the command is running", err)
		}
		if !st.Alive() {
			t.Error("the first command was stopped")
		}
	})

	t.Run("a command that ended is started again", func(t *testing.T) {
		base := t.TempDir()
		stateFile := filepath.Join(base, "api.pid")

		if _, err := startEntry(runbookfile.Entry{Name: "api", Run: cmdTrue}, base, stateFile, testAddr(base)); err != nil {
			t.Fatalf("startEntry(): %v", err)
		}
		st, _ := state.Read(stateFile)
		waitFor(t, func() bool { return !st.Alive() })

		if _, err := startEntry(runbookfile.Entry{Name: "api", Run: cmdSleep}, base, stateFile, testAddr(base)); err != nil {
			t.Errorf("startEntry() on a finished command: %v", err)
		}
		st, _ = state.Read(stateFile)
		t.Cleanup(func() { killGroup(st.Group()) })
	})
}

func TestStopEntry(t *testing.T) {
	t.Run("stops a running command and forgets it", func(t *testing.T) {
		st, stateFile, _ := startTest(t, cmdSleep)

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
		if _, err := os.Stat(stateFile); err == nil {
			t.Error("the state file is still there")
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
		if err := state.Write(stateFile, newState(0x7FFFFFFF, "1")); err != nil {
			t.Fatalf("state.Write(): %v", err)
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

func exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

// TestStartEntryBroadcasts is the whole of it: start puts a command and a
// broadcaster of its own behind an address, and what the command writes comes
// back out of it.
func TestStartEntryBroadcasts(t *testing.T) {
	_, _, base := startTest(t, cmdTick)

	conn, err := ipc.Dial(testAddr(base))
	if err != nil {
		t.Fatalf("ipc.Dial(): %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("reading what the command wrote: %v", err)
	}
	if got := string(buf[:n]); !strings.Contains(got, "tick") {
		t.Errorf("the command was heard saying %q, want it to hold %q", got, "tick")
	}
}
