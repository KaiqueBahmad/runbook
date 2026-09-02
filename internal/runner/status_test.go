package runner

import (
	"bytes"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"runbook/internal/ipc"
	"runbook/internal/runbookfile"
	"runbook/internal/state"
)

func TestStatus(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "runbook.yml")
	entries := []runbookfile.Entry{
		{Name: "web/server", Run: "sleep 30"},
		{Name: "never-started", Run: "sleep 30"},
		{Name: "gone", Run: "sleep 30"},
	}

	// One running, one never started, and one whose process is long gone.
	if _, err := startEntry(entries[0], project, state.File(path, "web/server"), ipc.Addr(path, "web/server")); err != nil {
		t.Fatalf("startEntry(): %v", err)
	}
	st, err := state.Read(state.File(path, "web/server"))
	if err != nil {
		t.Fatalf("state.Read(): %v", err)
	}
	t.Cleanup(func() { syscall.Kill(-st.Group(), syscall.SIGKILL) })

	if err := state.Write(state.File(path, "gone"), newState(0x7FFFFFFF, "1")); err != nil {
		t.Fatalf("state.Write(): %v", err)
	}

	found, err := Status(path, entries)
	if err != nil {
		t.Fatalf("Status(): %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("Status() = %+v, want only the running command", found)
	}
	if found[0].Name != "web/server" || found[0].PID != st.PID {
		t.Errorf("Status() = %+v, want web/server at pid %d", found[0], st.PID)
	}
}

func TestPrintStatus(t *testing.T) {
	found := []Running{
		{Name: "web/server", PID: 526142, Up: 12 * time.Second},
		{Name: "db", PID: 941, Up: 3*time.Minute + 4*time.Second},
	}

	tests := []struct {
		name  string
		found []Running
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
			PrintStatus(&out, tt.found, tt.align)
			if out.String() != tt.want {
				t.Errorf("PrintStatus() =\n%q\nwant\n%q", out.String(), tt.want)
			}
		})
	}
}
