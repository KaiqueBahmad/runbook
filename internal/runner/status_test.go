package runner

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"runbook/internal/ipc"
	"runbook/internal/runbookfile"
	"runbook/internal/state"
	"runbook/internal/workdir"
)

func TestStatus(t *testing.T) {
	path, store := testProject(t)
	project := filepath.Dir(path)
	entries := []runbookfile.Entry{
		{Name: "web/server", Run: cmdSleep},
		{Name: "never-started", Run: cmdSleep},
		{Name: "gone", Run: cmdSleep},
	}

	// One running, one never started, and one whose process is long gone.
	if _, err := startEntry(entries[0], project, state.File(store, "web/server"), ipc.Addr(store, "web/server")); err != nil {
		t.Fatalf("startEntry(): %v", err)
	}
	st, err := state.Read(state.File(store, "web/server"))
	if err != nil {
		t.Fatalf("state.Read(): %v", err)
	}
	t.Cleanup(func() { killGroup(st.Group()) })

	if err := state.Write(state.File(store, "gone"), newState(0x7FFFFFFF, "1")); err != nil {
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

func TestStatusAll(t *testing.T) {
	path, store := testProject(t)
	entry := runbookfile.Entry{Name: "web/server", Run: cmdSleep}

	if _, err := startEntry(entry, filepath.Dir(path), state.File(store, entry.Name), ipc.Addr(store, entry.Name)); err != nil {
		t.Fatalf("startEntry(): %v", err)
	}
	st, err := state.Read(state.File(store, entry.Name))
	if err != nil {
		t.Fatalf("state.Read(): %v", err)
	}
	t.Cleanup(func() { killGroup(st.Group()) })

	// Another runbook.yml, with nothing of it still running.
	other, err := workdir.Ensure(filepath.Join(t.TempDir(), "runbook.yml"))
	if err != nil {
		t.Fatalf("workdir.Ensure(): %v", err)
	}
	if err := state.Write(state.File(other, "gone"), newState(0x7FFFFFFF, "1")); err != nil {
		t.Fatalf("state.Write(): %v", err)
	}

	found, err := StatusAll()
	if err != nil {
		t.Fatalf("StatusAll(): %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("StatusAll() = %+v, want only the running command", found)
	}
	if r := found[0]; r.Name != entry.Name || r.PID != st.PID || r.File != path {
		t.Errorf("StatusAll() = %+v, want %s at pid %d of %s", r, entry.Name, st.PID, path)
	}
}

func TestPrintStatusAll(t *testing.T) {
	home := filepath.FromSlash("/home/someone")
	at := func(p string) string { return filepath.Join(home, filepath.FromSlash(p)) }
	found := []RunningIn{
		{Running: Running{Name: "web/server", PID: 526142, Up: 12 * time.Second}, File: at("projetos/api/runbook.yml")},
		{Running: Running{Name: "db", PID: 941, Up: 4 * time.Second}, File: at("projetos/api/other.yml")},
		{Running: Running{Name: "notes", PID: 4410, Up: time.Minute}, File: at("runbook.yml")},
		{Running: Running{Name: "old", PID: 77, Up: time.Second}, Work: at(".runbook/old-0123456789abcdef")},
	}
	sep := string(filepath.Separator)

	tests := []struct {
		name  string
		align bool
		want  string
	}{
		{
			// The project goes by its directory, a file of another name by
			// itself, and home is ~.
			name:  "aligned",
			align: true,
			want: "~" + sep + "projetos" + sep + "api            web/server  526142  12s\n" +
				"~" + sep + "projetos" + sep + "api" + sep + "other.yml  db             941  4s\n" +
				"~                         notes         4410  1m0s\n" +
				"old-0123456789abcdef      old             77  1s\n",
		},
		{
			name:  "not aligned",
			align: false,
			want: at("projetos/api/runbook.yml") + "\tweb/server\t526142\t12s\n" +
				at("projetos/api/other.yml") + "\tdb\t941\t4s\n" +
				at("runbook.yml") + "\tnotes\t4410\t1m0s\n" +
				"\told\t77\t1s\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			PrintStatusAll(&out, found, tt.align, home)
			if got := out.String(); got != tt.want {
				t.Errorf("PrintStatusAll() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
