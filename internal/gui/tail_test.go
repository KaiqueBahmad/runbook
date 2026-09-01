package gui

import (
	"fmt"
	"strings"
	"testing"
)

func TestTail(t *testing.T) {
	t.Run("a line at a time", func(t *testing.T) {
		var out tail
		fmt.Fprint(&out, "one\ntwo\n")

		if got := out.text(); got != "one\ntwo" {
			t.Errorf("text() = %q, want %q", got, "one\ntwo")
		}
	})

	t.Run("output that stops mid-line carries on", func(t *testing.T) {
		var out tail
		// A command's output arrives in pieces that have nothing to do with
		// where its lines end.
		fmt.Fprint(&out, "wai")
		fmt.Fprint(&out, "ting")
		if got := out.text(); got != "waiting" {
			t.Errorf("text() = %q, want the pieces on one line", got)
		}

		fmt.Fprint(&out, "...\ndone\n")
		if got := out.text(); got != "waiting...\ndone" {
			t.Errorf("text() = %q, want %q", got, "waiting...\ndone")
		}
	})

	t.Run("a blank line is a line", func(t *testing.T) {
		var out tail
		fmt.Fprint(&out, "one\n\ntwo\n")

		if got := out.text(); got != "one\n\ntwo" {
			t.Errorf("text() = %q, want the blank line kept", got)
		}
	})

	t.Run("only the last of it is kept", func(t *testing.T) {
		var out tail
		for i := range tailLines + 500 {
			fmt.Fprintf(&out, "line %d\n", i)
		}

		lines := strings.Split(out.text(), "\n")
		if len(lines) != tailLines {
			t.Fatalf("the tail holds %d lines, want %d", len(lines), tailLines)
		}
		if lines[0] != "line 500" {
			t.Errorf("the tail starts at %q, want the oldest to have gone", lines[0])
		}
		if want := fmt.Sprintf("line %d", tailLines+499); lines[len(lines)-1] != want {
			t.Errorf("the tail ends at %q, want %q", lines[len(lines)-1], want)
		}
	})

	t.Run("what Runbook says stands on its own line", func(t *testing.T) {
		var out tail
		fmt.Fprint(&out, "half a line")
		out.say("api ended with status %d", 0)

		want := "half a line\n[runbook] api ended with status 0"
		if got := out.text(); got != want {
			t.Errorf("text() = %q, want %q", got, want)
		}
	})

	t.Run("what has come in since it was drawn", func(t *testing.T) {
		var out tail
		if !out.empty() {
			t.Error("a command that has said nothing is not empty")
		}
		if out.fresh() {
			t.Error("fresh() before anything was written")
		}

		fmt.Fprint(&out, "hello\n")
		if !out.fresh() {
			t.Error("fresh() = false after something was written")
		}
		out.text()
		if out.fresh() {
			t.Error("fresh() = true after the window took the text")
		}
		if out.empty() {
			t.Error("empty() = true for a command that has said something")
		}
	})
}
