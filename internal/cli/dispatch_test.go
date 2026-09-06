package cli

import "testing"

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
