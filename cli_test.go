package main

import (
	"errors"
	"os"
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
		{"no args defaults to Runbookfile", nil, "", nil, abs(t, defaultRunbookfile), false},
		{"relative path is resolved", []string{"-f", "path/to/other"}, "", nil, abs(t, "path/to/other"), false},
		{"absolute path is kept", []string{"-f", "/etc/Runbookfile"}, "", nil, "/etc/Runbookfile", false},
		{"long flag", []string{"--file", "/etc/Runbookfile"}, "", nil, "/etc/Runbookfile", false},
		{"list", []string{"list"}, cmdList, nil, abs(t, defaultRunbookfile), false},
		{"list after the flag", []string{"-f", "/etc/Runbookfile", "list"}, cmdList, nil, "/etc/Runbookfile", false},
		{"list before the flag", []string{"list", "-f", "/etc/Runbookfile"}, cmdList, nil, "/etc/Runbookfile", false},
		{"completion", []string{"completion", "zsh"}, cmdCompletion, []string{"zsh"}, abs(t, defaultRunbookfile), false},
		{"run", []string{"run", "services/api"}, cmdRun, []string{"services/api"}, abs(t, defaultRunbookfile), false},
		{"run with the flag", []string{"run", "api", "-f", "/etc/Runbookfile"}, cmdRun, []string{"api"}, "/etc/Runbookfile", false},
		{"start", []string{"start", "services/api"}, cmdStart, []string{"services/api"}, abs(t, defaultRunbookfile), false},
		{"stop", []string{"stop", "services/api"}, cmdStop, []string{"services/api"}, abs(t, defaultRunbookfile), false},
		{"status", []string{"status"}, cmdStatus, nil, abs(t, defaultRunbookfile), false},
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

func TestCheckRunbookfile(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "Runbookfile")
	if err := os.WriteFile(file, []byte("api: echo hi\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", file, err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"existing file", file, false},
		{"missing file", filepath.Join(dir, "nope"), true},
		{"directory", dir, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkRunbookfile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkRunbookfile(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestEnsureRunbookDir(t *testing.T) {
	t.Run("creates the directory next to the runbookfile", func(t *testing.T) {
		project := t.TempDir()

		dir, err := ensureRunbookDir(filepath.Join(project, "Runbookfile"))
		if err != nil {
			t.Fatalf("ensureRunbookDir(): %v", err)
		}
		want := filepath.Join(project, runbookDirName)
		if dir != want {
			t.Errorf("ensureRunbookDir() = %q, want %q", dir, want)
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}

		got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if err != nil {
			t.Fatalf("reading .gitignore: %v", err)
		}
		if string(got) != gitignoreAll {
			t.Errorf(".gitignore = %q, want %q", got, gitignoreAll)
		}
	})

	t.Run("existing directory is kept", func(t *testing.T) {
		project := t.TempDir()

		dir := filepath.Join(project, runbookDirName)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
		file := filepath.Join(dir, "keep")
		if err := os.WriteFile(file, []byte("state\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", file, err)
		}

		if _, err := ensureRunbookDir(filepath.Join(project, "Runbookfile")); err != nil {
			t.Fatalf("ensureRunbookDir(): %v", err)
		}
		if _, err := os.Stat(file); err != nil {
			t.Errorf("stat %s: %v", file, err)
		}
	})

	t.Run("existing gitignore is kept", func(t *testing.T) {
		project := t.TempDir()

		dir := filepath.Join(project, runbookDirName)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
		ignore := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(ignore, []byte("logs/\n"), 0o600); err != nil {
			t.Fatalf("writing %s: %v", ignore, err)
		}

		if _, err := ensureRunbookDir(filepath.Join(project, "Runbookfile")); err != nil {
			t.Fatalf("ensureRunbookDir(): %v", err)
		}
		got, err := os.ReadFile(ignore)
		if err != nil {
			t.Fatalf("reading %s: %v", ignore, err)
		}
		if string(got) != "logs/\n" {
			t.Errorf(".gitignore = %q, want it left alone", got)
		}
	})

	t.Run("file in the way", func(t *testing.T) {
		project := t.TempDir()

		path := filepath.Join(project, runbookDirName)
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}

		if _, err := ensureRunbookDir(filepath.Join(project, "Runbookfile")); err == nil {
			t.Fatal("ensureRunbookDir() error = nil, want an error")
		}
	})
}
