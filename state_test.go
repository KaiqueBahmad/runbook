package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStatePaths(t *testing.T) {
	path := "/project/Runbookfile"

	t.Run("the state of one Runbookfile stays together", func(t *testing.T) {
		want := "/project/.runbook/Runbookfile.state"
		if got := stateDir(path); got != want {
			t.Errorf("stateDir() = %q, want %q", got, want)
		}
	})

	t.Run("folders in a name become directories", func(t *testing.T) {
		want := "/project/.runbook/Runbookfile.state/services/api.pid"
		if got := stateFile(path, "services/api"); got != want {
			t.Errorf("stateFile() = %q, want %q", got, want)
		}
	})

	t.Run("two Runbookfiles side by side do not share it", func(t *testing.T) {
		if stateDir(path) == stateDir("/project/Other") {
			t.Error("stateDir() is the same for two different Runbookfiles")
		}
	})
}

func TestState(t *testing.T) {
	file := filepath.Join(t.TempDir(), "deep", "api.pid")

	t.Run("a command that was never started", func(t *testing.T) {
		if _, err := readState(file); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("readState() error = %v, want it to be fs.ErrNotExist", err)
		}
	})

	t.Run("one line of three numbers", func(t *testing.T) {
		if err := writeState(file, state{PID: 557439, Boot: "11132527", Since: 1756699472}); err != nil {
			t.Fatalf("writeState(): %v", err)
		}
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		if string(got) != "557439 11132527 1756699472\n" {
			t.Errorf("the state file holds %q", got)
		}
	})

	t.Run("a file that is not one", func(t *testing.T) {
		for _, bad := range []string{"", "557439\n", "nope 1 2\n", "557439 1 later\n"} {
			if err := os.WriteFile(file, []byte(bad), 0o600); err != nil {
				t.Fatalf("writing %s: %v", file, err)
			}
			if _, err := readState(file); err == nil {
				t.Errorf("readState() of %q error = nil, want an error", bad)
			}
		}
	})

	t.Run("round trip", func(t *testing.T) {
		want := stateOf(4213, "10341905")
		if err := writeState(file, want); err != nil {
			t.Fatalf("writeState(): %v", err)
		}
		got, err := readState(file)
		if err != nil {
			t.Fatalf("readState(): %v", err)
		}
		if got != want {
			t.Errorf("readState() = %+v, want %+v", got, want)
		}
	})
}

func TestStateAlive(t *testing.T) {
	boot, err := processBoot(os.Getpid())
	if err != nil {
		t.Skipf("this system has no /proc: %v", err)
	}
	if boot == "" {
		t.Fatal("processBoot() is empty for the running process")
	}

	tests := []struct {
		name string
		st   state
		want bool
	}{
		{"the running process", state{PID: os.Getpid(), Boot: boot}, true},
		{"a process that is gone", state{PID: 0x7FFFFFFF, Boot: boot}, false},
		{"the number reused by another process", state{PID: os.Getpid(), Boot: "1"}, false},
		{"no start time recorded", state{PID: os.Getpid()}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.st.alive(); got != tt.want {
				t.Errorf("alive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStateSince(t *testing.T) {
	st := state{Since: time.Now().Add(-90 * time.Second).Unix()}
	if got := st.since(); got < 89*time.Second || got > 92*time.Second {
		t.Errorf("since() = %v, want about 90s", got)
	}
	if got := (state{}).since(); got != 0 {
		t.Errorf("since() = %v for an unset time, want 0", got)
	}
}

func TestSweep(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "Runbookfile")
	dir := stateDir(path)

	t.Run("a Runbookfile that never started anything", func(t *testing.T) {
		if err := sweep(path); err != nil {
			t.Errorf("sweep(): %v", err)
		}
	})

	write := func(t *testing.T, name, content string) string {
		t.Helper()
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(file), err)
		}
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %s: %v", file, err)
		}
		return file
	}

	t.Run("forgets what has ended and keeps the rest", func(t *testing.T) {
		boot, err := processBoot(os.Getpid())
		if err != nil {
			t.Skipf("this system has no /proc: %v", err)
		}

		gone := write(t, "web/server.pid", "2147483647 1 1756699472\n")
		alive := write(t, "db.pid", fmt.Sprintf("%d %s 1756699472\n", os.Getpid(), boot))
		// Written by a Runbook that records more than this one understands.
		unreadable := write(t, "future.pid", "941 1 1756699472 something-new\n")
		// Not a state file at all.
		other := write(t, "notes.txt", "hello\n")

		if err := sweep(path); err != nil {
			t.Fatalf("sweep(): %v", err)
		}

		if exists(gone) {
			t.Error("the state of a command that ended was kept")
		}
		if exists(filepath.Dir(gone)) {
			t.Error("the folder left empty by it was kept")
		}
		for _, file := range []string{alive, unreadable, other} {
			if !exists(file) {
				t.Errorf("%s was removed", filepath.Base(file))
			}
		}
	})

	t.Run("the state directory goes when the last of it does", func(t *testing.T) {
		for _, file := range []string{"db.pid", "future.pid", "notes.txt"} {
			if err := os.Remove(filepath.Join(dir, file)); err != nil {
				t.Fatalf("removing %s: %v", file, err)
			}
		}
		write(t, "one/two/three.pid", "2147483647 1 1756699472\n")

		if err := sweep(path); err != nil {
			t.Fatalf("sweep(): %v", err)
		}
		if exists(dir) {
			t.Errorf("%s was kept with nothing in it", dir)
		}
	})
}
