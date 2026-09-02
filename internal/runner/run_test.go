package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"runbook/internal/runbookfile"
)

func TestEntryDir(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"no dir is the runbook.yml's own directory", "", "/project"},
		{"a relative dir hangs off it", "services/api", "/project/services/api"},
		{"an absolute dir is kept", "/srv/api", "/srv/api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := entryDir(runbookfile.Entry{Dir: tt.dir}, "/project"); got != tt.want {
				t.Errorf("entryDir(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestEntryEnv(t *testing.T) {
	t.Run("no variables leaves the environment alone", func(t *testing.T) {
		if got := entryEnv(runbookfile.Entry{}); got != nil {
			t.Errorf("entryEnv() = %q, want nil", got)
		}
	})

	t.Run("variables are added on top", func(t *testing.T) {
		got := entryEnv(runbookfile.Entry{Env: map[string]string{"PORT": "8080"}})
		if !slices.Contains(got, "PORT=8080") {
			t.Errorf("entryEnv() = %q, want it to carry PORT=8080", got)
		}
		if len(got) != len(os.Environ())+1 {
			t.Errorf("entryEnv() has %d variables, want %d", len(got), len(os.Environ())+1)
		}
	})
}

func TestRunEntry(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "nested"), 0o755); err != nil {
		t.Fatalf("creating nested: %v", err)
	}

	tests := []struct {
		name  string
		entry runbookfile.Entry
		want  string // stdout, with the trailing newline trimmed
		code  int
	}{
		{"output", runbookfile.Entry{Name: "hello", Run: "echo hi"}, "hi", 0},
		{"status", runbookfile.Entry{Name: "fail", Run: "exit 3"}, "", 3},
		{"the shell reports an unknown command", runbookfile.Entry{Name: "nope", Run: "definitely-not-a-command"}, "", 127},
		{"runs in the runbook.yml's directory", runbookfile.Entry{Name: "here", Run: "pwd"}, base, 0},
		{"runs in dir", runbookfile.Entry{Name: "there", Run: "pwd", Dir: "nested"}, filepath.Join(base, "nested"), 0},
		{
			"passes the variables on",
			runbookfile.Entry{Name: "env", Run: "echo $PORT", Env: map[string]string{"PORT": "8080"}},
			"8080",
			0,
		},
		{"a signal reports 128 plus it", runbookfile.Entry{Name: "killed", Run: "kill -TERM $$"}, "", 143},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code, err := runEntry(tt.entry, base, &stdout, &stderr)
			if err != nil {
				t.Fatalf("runEntry(): %v\n%s", err, stderr.String())
			}
			if code != tt.code {
				t.Errorf("runEntry() = %d, want %d\n%s", code, tt.code, stderr.String())
			}
			if got := strings.TrimRight(stdout.String(), "\n"); got != tt.want {
				t.Errorf("runEntry() wrote %q, want %q", got, tt.want)
			}
		})
	}
}
