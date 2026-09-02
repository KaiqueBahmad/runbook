package cli

import (
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

// says runs Main with args and gives back what it printed and the status
// runbook would have exited with.
func says(t *testing.T, args ...string) (string, int) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	said := make(chan string, 1)
	go func() {
		var out strings.Builder
		io.Copy(&out, r)
		said <- out.String()
	}()

	code := Main(args)
	w.Close()
	return <-said, code
}

// TestIAmLLM is that the answer is there to be had from anywhere, whether or
// not the project has a runbook.yml yet — which is the moment a model asking
// what Runbook is has the most use for one.
func TestIAmLLM(t *testing.T) {
	t.Chdir(t.TempDir()) // nothing in it, least of all a runbook.yml

	said, code := says(t, "iamllm")
	if code != 0 {
		t.Errorf("runbook iamllm exited with %d, want it to work without a runbook.yml", code)
	}
	if said != primer {
		t.Errorf("runbook iamllm printed %q, want the primer", said)
	}
}

// TestPrimerCoversTheCommands is that the primer stays the whole story. What
// it leaves out is what the model reading it will never use, so a command
// added to Runbook and not to the primer is a command that model cannot reach.
func TestPrimerCoversTheCommands(t *testing.T) {
	// completion is for a shell to read rather than a model, and iamllm is
	// what the model has just run.
	quiet := []string{cmdCompletion, cmdIAmLLM}

	for _, cmd := range commands {
		if slices.Contains(quiet, cmd) {
			continue
		}
		if !strings.Contains(primer, "runbook "+cmd) {
			t.Errorf("the primer never mentions %q", "runbook "+cmd)
		}
	}
}
