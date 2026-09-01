package cli

import (
	"bytes"
	"testing"
)

func TestMainCommand(t *testing.T) {
	want := "parsed Runbookfile successfully\n"
	var out bytes.Buffer
	mainCommand(&out, "Runbookfile")
	if out.String() != want {
		t.Errorf("mainCommand() = %s, want %s", out.String(), want)
	}
}
