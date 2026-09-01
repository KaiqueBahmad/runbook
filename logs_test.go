package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogs(t *testing.T) {
	entries := []Entry{{Name: "api", Run: "sleep 30"}}

	t.Run("what is written", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "Runbookfile")
		w, b := serveTest(t, ipcAddr(path, "api"))

		in := invocation{cmd: cmdLogs, rest: []string{"api"}, path: path}
		var out bytes.Buffer
		done := make(chan error, 1)
		go func() { done <- logs(in, entries, &out) }()

		waitFor(t, func() bool { return listeners(b) == 1 })
		io.WriteString(w, "hello\n")
		w.Close() // the command has ended, so logs is done

		if err := <-done; err != nil {
			t.Fatalf("logs(): %v", err)
		}
		if got := out.String(); got != "hello\n" {
			t.Errorf("logs() wrote %q, want %q", got, "hello\n")
		}
	})

	t.Run("not running", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "Runbookfile")
		in := invocation{cmd: cmdLogs, rest: []string{"api"}, path: path}

		err := logs(in, entries, io.Discard)
		if err == nil {
			t.Fatal("logs() error = nil, want an error")
		}
		if !strings.Contains(err.Error(), "start api") {
			t.Errorf("logs() error = %v, want it to point at starting the command", err)
		}
	})

	t.Run("unknown name", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "Runbookfile")
		in := invocation{cmd: cmdLogs, rest: []string{"web"}, path: path}

		err := logs(in, entries, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "no command named") {
			t.Errorf("logs() error = %v, want it to say there is no such command", err)
		}
	})
}
