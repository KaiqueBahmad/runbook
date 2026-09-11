package runner

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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
	entries := []runbookfile.Entry{{Name: "api", Run: cmdSleep}}

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

func TestLogsAll(t *testing.T) {
	entries := []runbookfile.Entry{
		{Name: "api", Run: cmdSleep},
		{Name: "web", Run: cmdSleep},
		{Name: "idle", Run: cmdSleep},
	}

	t.Run("all of them", func(t *testing.T) {
		path, store := testProject(t)
		api := broadcastTest(t, ipc.Addr(store, "api"))
		web := broadcastTest(t, ipc.Addr(store, "web"))

		var out heard
		done := make(chan error, 1)
		go func() { done <- LogsAll(path, entries, &out, true) }()

		// Both keep talking until both are heard: what either says before
		// LogsAll has connected is nobody's.
		waitFor(t, func() bool {
			io.WriteString(api, "from the api\n")
			io.WriteString(web, "from the web\n")
			got := out.String()
			return strings.Contains(got, "from the api") && strings.Contains(got, "from the web")
		})

		// It waits for the last of them: with only one ended there is still
		// something to hear.
		api.Close()
		select {
		case err := <-done:
			t.Fatalf("LogsAll() returned with web still going: %v", err)
		case <-time.After(100 * time.Millisecond):
		}

		web.Close()
		if err := <-done; err != nil {
			t.Fatalf("LogsAll(): %v", err)
		}

		got := out.String()
		// The names line up in a column, the shorter padded out to the longer.
		if !strings.Contains(got, "api  from the api\n") {
			t.Errorf("LogsAll() wrote %q, want it to hold %q", got, "api  from the api\n")
		}
		if !strings.Contains(got, "web  from the web\n") {
			t.Errorf("LogsAll() wrote %q, want it to hold %q", got, "web  from the web\n")
		}
		// A command that was never started is not among them.
		if strings.Contains(got, "idle") {
			t.Errorf("LogsAll() wrote %q, want nothing of a command that is not running", got)
		}
	})

	t.Run("half a line", func(t *testing.T) {
		path, store := testProject(t)
		api := broadcastTest(t, ipc.Addr(store, "api"))

		var out heard
		done := make(chan error, 1)
		go func() { done <- LogsAll(path, entries, &out, false) }()

		// A whole line first, and waited for: what the command says before
		// LogsAll has connected is nobody's, half a line or not.
		waitFor(t, func() bool {
			io.WriteString(api, "a whole line\n")
			return out.String() != ""
		})
		io.WriteString(api, "half a line")
		api.Close() // the command has ended, so what it left is all there is

		if err := <-done; err != nil {
			t.Fatalf("LogsAll(): %v", err)
		}
		// Not aligned, a tab separates the name from the line, so that
		// something other than a person can take it apart.
		if got := out.String(); !strings.Contains(got, "api\thalf a line\n") {
			t.Errorf("LogsAll() wrote %q, want it to hold %q", got, "api\thalf a line\n")
		}
	})

	t.Run("none running", func(t *testing.T) {
		path, _ := testProject(t)

		err := LogsAll(path, entries, io.Discard, true)
		if err == nil {
			t.Fatal("LogsAll() error = nil, want an error")
		}
		if !strings.Contains(err.Error(), "nothing is broadcasting") {
			t.Errorf("LogsAll() error = %v, want it to say nothing is broadcasting", err)
		}
	})
}
