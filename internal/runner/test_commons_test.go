package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"runbook/internal/ipc"
	"runbook/internal/workdir"
)

// TestMain sends the test binary to broadcast when it is started as one, which
// is what startEntry does: it starts another copy of runbook to carry a
// command's output, and under test the copy at hand is this binary.
func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == BroadcastCommand {
		if err := ipc.Broadcast(os.Args[2], os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// testProject writes a runbook.yml in a directory of its own and gives back
// its path along with the directory Runbook keeps its files in. The home
// directory is one of the test's own, so that running the tests leaves nothing
// in the home directory of whoever ran them.
func testProject(t *testing.T) (string, string) {
	t.Helper()

	t.Setenv(envHome, t.TempDir())
	path := filepath.Join(t.TempDir(), "runbook.yml")

	store, err := workdir.Ensure(path)
	if err != nil {
		t.Fatalf("workdir.Ensure(): %v", err)
	}
	return path, store
}

// waitFor gives a condition a couple of seconds to come true, for the moments
// where a command Runbook started has to get somewhere first.
func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	for range 100 {
		if done() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("waited two seconds and it never happened")
}
