//go:build !windows

package runner

import (
	"os/exec"
	"testing"
)

// TestCdTo is that the quoting holds: sh moves into the very directory named,
// however awkward its name.
func TestCdTo(t *testing.T) {
	for _, name := range []string{"plain", "with space", "it's", `a"b`, "$HOME"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir() + "/" + name
			if out, err := exec.Command("mkdir", dir).CombinedOutput(); err != nil {
				t.Fatalf("mkdir: %v\n%s", err, out)
			}
			out, err := exec.Command(shell, shellFlag, cdTo(dir)+" && pwd").Output()
			if err != nil {
				t.Fatalf("%s: %v", cdTo(dir), err)
			}
			if got := string(out); got != dir+"\n" {
				t.Errorf("%s ended up in %q, want %q", cdTo(dir), got, dir)
			}
		})
	}
}
