package state

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
	// What Runbook keeps for one project, which is where the state of its
	// commands goes.
	work := "/home/someone/.runbook/project-0123456789abcdef"

	t.Run("the state of one project stays together", func(t *testing.T) {
		want := work + "/state"
		if got := Dir(work); got != want {
			t.Errorf("Dir() = %q, want %q", got, want)
		}
	})

	t.Run("folders in a name become directories", func(t *testing.T) {
		want := work + "/state/services/api.pid"
		if got := File(work, "services/api"); got != want {
			t.Errorf("File() = %q, want %q", got, want)
		}
	})
}

func TestState(t *testing.T) {
	file := filepath.Join(t.TempDir(), "deep", "api.pid")

	t.Run("a command that was never started", func(t *testing.T) {
		if _, err := Read(file); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Read() error = %v, want it to be fs.ErrNotExist", err)
		}
	})

	t.Run("one line of three numbers", func(t *testing.T) {
		if err := Write(file, State{PID: 557439, Boot: "11132527", Since: 1756699472}); err != nil {
			t.Fatalf("Write(): %v", err)
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
			if _, err := Read(file); err == nil {
				t.Errorf("Read() of %q error = nil, want an error", bad)
			}
		}
	})

	t.Run("round trip", func(t *testing.T) {
		want := State{PID: 4213, Boot: "10341905", Since: time.Now().Unix()}
		if err := Write(file, want); err != nil {
			t.Fatalf("Write(): %v", err)
		}
		got, err := Read(file)
		if err != nil {
			t.Fatalf("Read(): %v", err)
		}
		if got != want {
			t.Errorf("Read() = %+v, want %+v", got, want)
		}
	})
}

func TestStateAlive(t *testing.T) {
	boot, err := ProcessBoot(os.Getpid())
	if err != nil {
		t.Skipf("this system has no /proc: %v", err)
	}
	if boot == "" {
		t.Fatal("ProcessBoot() is empty for the running process")
	}

	tests := []struct {
		name string
		st   State
		want bool
	}{
		{"the running process", State{PID: os.Getpid(), Boot: boot}, true},
		{"a process that is gone", State{PID: 0x7FFFFFFF, Boot: boot}, false},
		{"the number reused by another process", State{PID: os.Getpid(), Boot: "1"}, false},
		{"no start time recorded", State{PID: os.Getpid()}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.st.Alive(); got != tt.want {
				t.Errorf("Alive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStateUptime(t *testing.T) {
	st := State{Since: time.Now().Add(-90 * time.Second).Unix()}
	if got := st.Uptime(); got < 89*time.Second || got > 92*time.Second {
		t.Errorf("Uptime() = %v, want about 90s", got)
	}
	if got := (State{}).Uptime(); got != 0 {
		t.Errorf("Uptime() = %v for an unset time, want 0", got)
	}
}

func TestSweep(t *testing.T) {
	work := t.TempDir()
	dir := Dir(work)

	t.Run("a project that never started anything", func(t *testing.T) {
		if err := Sweep(work); err != nil {
			t.Errorf("Sweep(): %v", err)
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
		boot, err := ProcessBoot(os.Getpid())
		if err != nil {
			t.Skipf("this system has no /proc: %v", err)
		}

		gone := write(t, "web/server.pid", "2147483647 1 1756699472\n")
		alive := write(t, "db.pid", fmt.Sprintf("%d %s 1756699472\n", os.Getpid(), boot))
		// Written by a Runbook that records more than this one understands.
		unreadable := write(t, "future.pid", "941 1 1756699472 something-new\n")
		// Not a state file at all.
		other := write(t, "notes.txt", "hello\n")

		if err := Sweep(work); err != nil {
			t.Fatalf("Sweep(): %v", err)
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

		if err := Sweep(work); err != nil {
			t.Fatalf("Sweep(): %v", err)
		}
		if exists(dir) {
			t.Errorf("%s was kept with nothing in it", dir)
		}
	})
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}
