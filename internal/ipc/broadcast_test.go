package ipc

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testAddr is an address to broadcast on, kept short: a unix socket address is
// a path, and the kernel takes about a hundred characters of it.
func testAddr(base string) string {
	return filepath.Join(base, "api.sock")
}

// serveTest broadcasts on addr and gives back the writer that stands in for
// the command, and the broadcaster behind it, so a test can wait for a
// listener to have connected before writing anything for it to hear.
func serveTest(t *testing.T, addr string) (*os.File, *broadcaster) {
	t.Helper()

	l, err := Listen(addr)
	if err != nil {
		t.Fatalf("Listen(%q): %v", addr, err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}

	b := newBroadcaster()
	go b.accept(l)
	go func() {
		b.drain(r)
		b.end()
		l.Close()
		r.Close()
	}()

	t.Cleanup(func() { w.Close() })
	return w, b
}

// listeners is how many are connected, for a test that has to wait for one.
func listeners(b *broadcaster) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.listeners)
}

// listen connects to a broadcaster and reads everything it says, giving it back
// once the command has ended and the connection is closed from the other side.
func listen(t *testing.T, addr string) <-chan string {
	t.Helper()

	conn, err := Dial(addr)
	if err != nil {
		t.Fatalf("Dial(%q): %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() })

	heard := make(chan string, 1)
	go func() {
		var out bytes.Buffer
		io.Copy(&out, conn)
		heard <- out.String()
	}()
	return heard
}

func TestBroadcast(t *testing.T) {
	t.Run("everyone listening hears the same thing", func(t *testing.T) {
		addr := testAddr(t.TempDir())
		w, b := serveTest(t, addr)

		first := listen(t, addr)
		second := listen(t, addr)
		waitFor(t, func() bool { return listeners(b) == 2 })

		io.WriteString(w, "hello\n")
		w.Close() // the command has ended

		for i, heard := range []<-chan string{first, second} {
			if got := <-heard; got != "hello\n" {
				t.Errorf("listener %d heard %q, want %q", i, got, "hello\n")
			}
		}
	})

	t.Run("a listener hears nothing of what came before it", func(t *testing.T) {
		addr := testAddr(t.TempDir())
		w, b := serveTest(t, addr)

		// The first listener is there to say when the broadcaster has taken the
		// output in: once it has heard something, that something is past, and a
		// listener connecting afterwards has missed it for good.
		first, err := Dial(addr)
		if err != nil {
			t.Fatalf("Dial(%q): %v", addr, err)
		}
		defer first.Close()
		waitFor(t, func() bool { return listeners(b) == 1 })

		io.WriteString(w, "before\n")
		first.SetReadDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 64)
		n, err := first.Read(buf)
		if err != nil {
			t.Fatalf("the first listener heard nothing: %v", err)
		}
		if got := string(buf[:n]); got != "before\n" {
			t.Fatalf("the first listener heard %q, want %q", got, "before\n")
		}

		late := listen(t, addr)
		waitFor(t, func() bool { return listeners(b) == 2 })
		io.WriteString(w, "after\n")
		w.Close()

		if got := <-late; got != "after\n" {
			t.Errorf("the late listener heard %q, want only %q", got, "after\n")
		}
	})

	t.Run("a command whose output nobody wants keeps going", func(t *testing.T) {
		addr := testAddr(t.TempDir())
		w, _ := serveTest(t, addr)

		// More than a pipe holds, so a broadcaster that stopped reading, or
		// kept what it read, would leave this stuck in the write.
		done := make(chan struct{})
		go func() {
			io.WriteString(w, strings.Repeat("x", 1<<20))
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("the command is stuck writing output nobody is listening to")
		}
	})

	t.Run("a listener that stops reading is let go", func(t *testing.T) {
		b := newBroadcaster()
		mine, theirs := net.Pipe() // nothing ever reads mine
		defer mine.Close()
		defer theirs.Close()
		b.add(theirs)

		// Every one of these has to come back rather than wait on the listener.
		for range backlog + 2 {
			b.send([]byte("x"))
		}
		if n := listeners(b); n != 0 {
			t.Errorf("%d listeners left, want the one that stopped reading to have been let go", n)
		}
	})

	t.Run("the listeners are let go when the command ends", func(t *testing.T) {
		addr := testAddr(t.TempDir())
		w, b := serveTest(t, addr)

		heard := listen(t, addr)
		waitFor(t, func() bool { return listeners(b) == 1 })

		io.WriteString(w, "last words\n")
		w.Close()

		select {
		case got := <-heard:
			if got != "last words\n" {
				t.Errorf("the listener heard %q, want %q", got, "last words\n")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the listener was left hanging on a command that has ended")
		}
	})

	t.Run("connecting to a command that is not running", func(t *testing.T) {
		if _, err := Dial(testAddr(t.TempDir())); err == nil {
			t.Error("Dial() error = nil, want it to fail where nothing is listening")
		}
	})
}

// waitFor gives a condition a couple of seconds to come true, for the moments
// where the other side of a connection has to get somewhere first.
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

func exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}
