package runner

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"runbook/internal/ipc"
	"runbook/internal/runbookfile"
)

// heard is what logs has written so far, from a goroutine of its own.
type heard struct {
	mu  sync.Mutex
	out bytes.Buffer
}

func (h *heard) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.out.Write(p)
}

func (h *heard) String() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.out.String()
}

// broadcastTest stands in for a started command: it broadcasts on the address
// of one, and gives back the writer that stands in for the command's output.
func broadcastTest(t *testing.T, addr string) *os.File {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}
	go func() {
		ipc.Broadcast(addr, r)
		r.Close()
	}()
	waitFor(t, func() bool {
		conn, err := ipc.Dial(addr)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	})

	t.Cleanup(func() { w.Close() })
	return w
}

func TestLogs(t *testing.T) {
	entries := []runbookfile.Entry{{Name: "api", Run: "sleep 30"}}

	t.Run("what is written", func(t *testing.T) {
		path, store := testProject(t)
		w := broadcastTest(t, ipc.Addr(store, "api"))

		var out heard
		done := make(chan error, 1)
		go func() { done <- Logs(path, entries, "api", &out) }()

		// The command keeps talking until it is heard, so that nothing is said
		// in the moment before logs has connected.
		waitFor(t, func() bool {
			io.WriteString(w, "hello\n")
			return out.String() != ""
		})
		w.Close() // the command has ended, so logs is done

		if err := <-done; err != nil {
			t.Fatalf("Logs(): %v", err)
		}
		if got := out.String(); !strings.Contains(got, "hello\n") {
			t.Errorf("Logs() wrote %q, want it to hold %q", got, "hello\n")
		}
	})

	t.Run("not running", func(t *testing.T) {
		path, _ := testProject(t)

		err := Logs(path, entries, "api", io.Discard)
		if err == nil {
			t.Fatal("Logs() error = nil, want an error")
		}
		if !strings.Contains(err.Error(), "start api") {
			t.Errorf("Logs() error = %v, want it to point at starting the command", err)
		}
	})

	t.Run("unknown name", func(t *testing.T) {
		path, _ := testProject(t)

		err := Logs(path, entries, "web", io.Discard)
		if err == nil || !strings.Contains(err.Error(), "no command named") {
			t.Errorf("Logs() error = %v, want it to say there is no such command", err)
		}
	})
}
