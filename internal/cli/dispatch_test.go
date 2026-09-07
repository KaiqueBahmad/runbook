package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"runbook/internal/runbookfile"
)

// TestNoCommandPrintsHelp is that nothing opens on its own: the panel is
// runbook gui and nothing else, so runbook on its own has only its help to
// give. It is run where there is no runbook.yml, which is also where opening
// a window would have had nothing to show.
func TestNoCommandPrintsHelp(t *testing.T) {
	t.Chdir(t.TempDir())

	said, code := says(t)
	if code != 0 {
		t.Errorf("runbook exited with %d, want 0", code)
	}
	if said != help+"\n" {
		t.Errorf("runbook printed %q, want the help", said)
	}
}

// TestListFromBelowTheRunbook is the walk up, end to end: someone deep inside a
// project asks for its commands and gets them, without having to climb back to
// the directory the runbook.yml sits in first.
func TestListFromBelowTheRunbook(t *testing.T) {
	project := t.TempDir()

	deep := filepath.Join(project, "services", "api")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("creating %s: %v", deep, err)
	}
	file := filepath.Join(project, runbookfile.Name)
	if err := os.WriteFile(file, []byte("api:\n  run: npm start\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", file, err)
	}
	t.Chdir(deep)

	said, code := says(t, "list")
	if code != 0 {
		t.Errorf("runbook list exited with %d, want 0", code)
	}
	if got := strings.TrimSpace(said); got != "api" {
		t.Errorf("runbook list printed %q, want %q", got, "api")
	}
}

// TestListWithNoRunbookAnywhere is that the walk gives up rather than going on
// forever, and says where it looked.
func TestListWithNoRunbookAnywhere(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if _, code := says(t, "list"); code != 1 {
		t.Errorf("runbook list exited with %d, want 1", code)
	}
}
