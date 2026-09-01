package gui

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
)

// tailLines is how much of what a command has said the window keeps. What
// falls off the top is gone: Runbook writes nothing down, and a window left
// open on a chatty command would grow all day.
const tailLines = 1000

// tail is the last of what one command has written. The goroutine listening to
// the command writes to it and the one drawing the window reads it, so all of
// it is behind the lock.
type tail struct {
	mu      sync.Mutex
	lines   []string
	open    bool // the last line is still being written
	changed bool // something has come in since the window last drew it
}

// Write takes what a command said, a line at a time. Output arrives in pieces
// that have nothing to do with where the lines end, so a piece that ends
// mid-line leaves that line open for the next one to carry on.
func (t *tail) Write(p []byte) (int, error) {
	n := len(p)

	t.mu.Lock()
	defer t.mu.Unlock()

	for {
		end := bytes.IndexByte(p, '\n')
		if end < 0 {
			if len(p) > 0 {
				t.write(string(p))
			}
			break
		}
		t.write(string(p[:end]))
		t.open = false
		p = p[end+1:]
	}
	t.changed = true
	return n, nil
}

// write adds to the line being written, starting one if the last was finished,
// and forgets the oldest once there is more than the window keeps.
func (t *tail) write(s string) {
	if !t.open {
		t.lines = append(t.lines, "")
		t.open = true
		if len(t.lines) > tailLines {
			t.lines = append(t.lines[:0], t.lines[len(t.lines)-tailLines:]...)
		}
	}
	t.lines[len(t.lines)-1] += s
}

// say is Runbook putting a line of its own among a command's output, for what
// only the window knows: that a command was started here, or how it ended.
func (t *tail) say(format string, args ...any) {
	t.mu.Lock()
	// Whatever was half written stands on its own rather than being run into.
	t.open = false
	t.mu.Unlock()

	fmt.Fprintf(t, "[runbook] "+format+"\n", args...)
}

// text is everything the tail holds, and taking it counts as having drawn it.
func (t *tail) text() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.changed = false
	return strings.Join(t.lines, "\n")
}

// fresh reports whether anything has come in since the text was last taken.
func (t *tail) fresh() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.changed
}

// empty reports whether the command has said nothing there is to show.
func (t *tail) empty() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.lines) == 0
}
