package main

import (
	"bytes"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestStatusOf(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "Runbookfile")
	entries := []Entry{
		{Name: "web/server", Run: "sleep 30"},
		{Name: "never-started", Run: "sleep 30"},
		{Name: "gone", Run: "sleep 30"},
	}

	// One running, one never started, and one whose process is long gone.
	if _, err := startEntry(entries[0], project, stateFile(path, "web/server"), ipcAddr(path, "web/server")); err != nil {
		t.Fatalf("startEntry(): %v", err)
	}
	st, err := readState(stateFile(path, "web/server"))
	if err != nil {
		t.Fatalf("readState(): %v", err)
	}
	t.Cleanup(func() { syscall.Kill(-st.group(), syscall.SIGKILL) })

	if err := writeState(stateFile(path, "gone"), stateOf(0x7FFFFFFF, "1")); err != nil {
		t.Fatalf("writeState(): %v", err)
	}

	found, err := statusOf(path, entries)
	if err != nil {
		t.Fatalf("statusOf(): %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("statusOf() = %+v, want only the running command", found)
	}
	if found[0].name != "web/server" || found[0].pid != st.PID {
		t.Errorf("statusOf() = %+v, want web/server at pid %d", found[0], st.PID)
	}
}

func TestPrintStatus(t *testing.T) {
	found := []running{
		{name: "web/server", pid: 526142, up: 12 * time.Second},
		{name: "db", pid: 941, up: 3*time.Minute + 4*time.Second},
	}

	tests := []struct {
		name  string
		found []running
		align bool
		want  string
	}{
		{
			// The names line up on the left, the process ids on the right.
			name:  "aligned",
			found: found,
			align: true,
			want: "web/server  526142  12s\n" +
				"db             941  3m4s\n",
		},
		{
			name:  "not aligned",
			found: found,
			align: false,
			want:  "web/server\t526142\t12s\ndb\t941\t3m4s\n",
		},
		{
			name:  "nothing running says so to a person",
			found: nil,
			align: true,
			want:  "nothing running\n",
		},
		{
			name:  "and says nothing at all to anything else",
			found: nil,
			align: false,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			printStatus(&out, tt.found, tt.align)
			if out.String() != tt.want {
				t.Errorf("printStatus() =\n%q\nwant\n%q", out.String(), tt.want)
			}
		})
	}
}
