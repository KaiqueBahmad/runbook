package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionScript(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			script := completionScript(shell)
			if script == "" {
				t.Fatalf("completionScript(%q) is empty", shell)
			}
			if !strings.Contains(script, "runbook") {
				t.Errorf("completionScript(%q) never mentions runbook", shell)
			}
		})
	}

	t.Run("unknown shell", func(t *testing.T) {
		if got := completionScript("csh"); got != "" {
			t.Errorf("completionScript(\"csh\") = %q, want it empty", got)
		}
	})
}

// TestCompletionScriptParses runs each script through its own shell's syntax
// check, so a broken script fails here rather than in somebody's terminal. A
// shell that is not installed is skipped.
func TestCompletionScriptParses(t *testing.T) {
	// The flags that make a shell parse a file without running it.
	checks := map[string][]string{
		"bash": {"-n"},
		"zsh":  {"-n"},
		"fish": {"--no-execute"},
	}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			bin, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s is not installed", shell)
			}

			path := filepath.Join(t.TempDir(), "completion."+shell)
			if err := os.WriteFile(path, []byte(completionScript(shell)), 0o600); err != nil {
				t.Fatalf("writing %s: %v", path, err)
			}

			out, err := exec.Command(bin, append(checks[shell], path)...).CombinedOutput()
			if err != nil {
				t.Errorf("%s %v: %v\n%s", shell, checks[shell], err, out)
			}
		})
	}
}

// TestBashCompletionSuggests drives the bash function the way bash itself
// does, so what the script offers at each position is pinned down.
func TestBashCompletionSuggests(t *testing.T) {
	bin, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not installed")
	}

	path := filepath.Join(t.TempDir(), "completion.bash")
	if err := os.WriteFile(path, []byte(completionScript("bash")), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	// Load the script, fill in what bash would, and print the suggestions.
	const driver = `source "$1"; shift; COMP_WORDS=("$@"); COMP_CWORD=$(($# - 1)); _runbook; echo "${COMPREPLY[*]}"`

	tests := []struct {
		name  string
		words []string // the command line, ending with the word being completed
		want  string
	}{
		{"a command", []string{"runbook", ""}, "list run start stop status logs completion"},
		{"a half typed command", []string{"runbook", "l"}, "list logs"},
		{"a command after the flag", []string{"runbook", "-f", "runbook.yml", ""}, "list run start stop status logs completion"},
		{"the shell completion takes", []string{"runbook", "completion", ""}, "bash zsh fish"},
		{"a half typed shell", []string{"runbook", "completion", "z"}, "zsh"},
		{"nothing after list", []string{"runbook", "list", ""}, ""},
		{"nothing after the shell", []string{"runbook", "completion", "bash", ""}, ""},
		{"the flags", []string{"runbook", "-"}, "-f --file -h --help"},
		{"the names run takes", []string{"runbook", "run", ""}, "services/api lint"},
		{"a half typed name", []string{"runbook", "run", "l"}, "lint"},
		{"nothing after the name", []string{"runbook", "run", "lint", ""}, ""},
		{"the names start takes", []string{"runbook", "start", ""}, "services/api lint"},
		{"the names stop takes", []string{"runbook", "stop", ""}, "services/api lint"},
		{"the names logs takes", []string{"runbook", "logs", ""}, "services/api lint"},
	}

	// The script asks the runbook being completed for the names, so put one on
	// the PATH that answers.
	stub := filepath.Join(t.TempDir(), "runbook")
	const answer = "#!/bin/sh\nprintf 'services/api\\tThe Spring backend\\n'\nprintf 'lint\\n'\n"
	if err := os.WriteFile(stub, []byte(answer), 0o700); err != nil {
		t.Fatalf("writing %s: %v", stub, err)
	}
	env := append(os.Environ(), "PATH="+filepath.Dir(stub)+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-c", driver, "bash", path}, tt.words...)
			cmd := exec.Command(bin, args...)
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			if got := strings.TrimSpace(string(out)); got != tt.want {
				t.Errorf("%q completes to %q, want %q", tt.words, got, tt.want)
			}
		})
	}
}
