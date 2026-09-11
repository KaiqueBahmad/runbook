//go:build !windows

package runner

import (
	"bytes"
	"testing"

	"runbook/internal/runbookfile"
)

// TestRunEntrySignal is a command killed by a signal reporting 128 plus that
// signal, the way a shell does. Windows has no signals, so it has no part in
// the table the rest of runEntry is tested by.
func TestRunEntrySignal(t *testing.T) {
	var stdout, stderr bytes.Buffer
	entry := runbookfile.Entry{Name: "killed", Run: "kill -TERM $$"}

	code, err := runEntry(entry, t.TempDir(), &stdout, &stderr)
	if err != nil {
		t.Fatalf("runEntry(): %v\n%s", err, stderr.String())
	}
	if code != 143 {
		t.Errorf("runEntry() = %d, want 143", code)
	}
}
