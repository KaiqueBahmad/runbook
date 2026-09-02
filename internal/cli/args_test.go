package cli

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	abs := func(t *testing.T, p string) string {
		t.Helper()
		full, err := filepath.Abs(p)
		if err != nil {
			t.Fatalf("filepath.Abs(%q): %v", p, err)
		}
		return full
	}

	tests := []struct {
		name     string
		args     []string
		wantCmd  string
		wantRest []string
		want     string
		wantErr  bool
	}{
		{"no args defaults to runbook.yml", nil, "", nil, abs(t, defaultFile), false},
		{"relative path is resolved", []string{"-f", "path/to/other"}, "", nil, abs(t, "path/to/other"), false},
		{"absolute path is kept", []string{"-f", "/etc/runbook.yml"}, "", nil, "/etc/runbook.yml", false},
		{"long flag", []string{"--file", "/etc/runbook.yml"}, "", nil, "/etc/runbook.yml", false},
		{"list", []string{"list"}, cmdList, nil, abs(t, defaultFile), false},
		{"list after the flag", []string{"-f", "/etc/runbook.yml", "list"}, cmdList, nil, "/etc/runbook.yml", false},
		{"list before the flag", []string{"list", "-f", "/etc/runbook.yml"}, cmdList, nil, "/etc/runbook.yml", false},
		{"completion", []string{"completion", "zsh"}, cmdCompletion, []string{"zsh"}, abs(t, defaultFile), false},
		{"run", []string{"run", "services/api"}, cmdRun, []string{"services/api"}, abs(t, defaultFile), false},
		{"run with the flag", []string{"run", "api", "-f", "/etc/runbook.yml"}, cmdRun, []string{"api"}, "/etc/runbook.yml", false},
		{"start", []string{"start", "services/api"}, cmdStart, []string{"services/api"}, abs(t, defaultFile), false},
		{"stop", []string{"stop", "services/api"}, cmdStop, []string{"services/api"}, abs(t, defaultFile), false},
		{"status", []string{"status"}, cmdStatus, nil, abs(t, defaultFile), false},
		{"logs", []string{"logs", "services/api"}, cmdLogs, []string{"services/api"}, abs(t, defaultFile), false},
		{"broadcast takes an address", []string{"broadcast", "/tmp/api.sock"}, cmdBroadcast, []string{"/tmp/api.sock"}, abs(t, defaultFile), false},
		{"empty path", []string{"-f", ""}, "", nil, "", true},
		{"path without the flag", []string{"path/to/other"}, "", nil, "", true},
		{"flag without a path", []string{"-f"}, "", nil, "", true},
		{"flag given twice", []string{"-f", "a", "-f", "b"}, "", nil, "", true},
		{"unknown flag", []string{"--nope", "list"}, "", nil, "", true},
		{"unknown command", []string{"run"}, "", nil, "", true},
		{"command given twice", []string{"list", "list"}, "", nil, "", true},
		{"too many args", []string{"-f", "a", "list", "b"}, "", nil, "", true},
		{"completion without a shell", []string{"completion"}, "", nil, "", true},
		{"completion with an unknown shell", []string{"completion", "csh"}, "", nil, "", true},
		{"completion with two shells", []string{"completion", "zsh", "fish"}, "", nil, "", true},
		{"run without a name", []string{"run"}, "", nil, "", true},
		{"run with two names", []string{"run", "api", "web"}, "", nil, "", true},
		{"start without a name", []string{"start"}, "", nil, "", true},
		{"stop without a name", []string{"stop"}, "", nil, "", true},
		{"logs without a name", []string{"logs"}, "", nil, "", true},
		{"logs with two names", []string{"logs", "api", "web"}, "", nil, "", true},
		{"broadcast without an address", []string{"broadcast"}, "", nil, "", true},
		{"status with a name", []string{"status", "api"}, "", nil, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs(%q) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if got.cmd != tt.wantCmd {
				t.Errorf("parseArgs(%q) command = %q, want %q", tt.args, got.cmd, tt.wantCmd)
			}
			if !slices.Equal(got.rest, tt.wantRest) {
				t.Errorf("parseArgs(%q) rest = %q, want %q", tt.args, got.rest, tt.wantRest)
			}
			if got.path != tt.want {
				t.Errorf("parseArgs(%q) = %q, want %q", tt.args, got.path, tt.want)
			}
		})
	}
}

func TestParseArgsHelp(t *testing.T) {
	args := [][]string{
		{"--help"},
		{"-h"},
		{"-f", "path/to/other", "--help"},
		{"list", "--help"},
		{"-h", "a", "b"},
	}

	for _, tt := range args {
		t.Run(strings.Join(tt, " "), func(t *testing.T) {
			got, err := parseArgs(tt)
			if !errors.Is(err, errHelpRequested) {
				t.Fatalf("parseArgs(%q) error = %v, want errHelpRequested", tt, err)
			}
			if got.cmd != "" || got.path != "" {
				t.Errorf("parseArgs(%q) = %q, %q, want both empty", tt, got.cmd, got.path)
			}
		})
	}
}
