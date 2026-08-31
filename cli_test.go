package main

import (
	"errors"
	"os"
	"path/filepath"
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
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{"no args defaults to Runbookfile", nil, abs(t, defaultRunbookfile), false},
		{"relative path is resolved", []string{"path/to/other"}, abs(t, "path/to/other"), false},
		{"absolute path is kept", []string{"/etc/Runbookfile"}, "/etc/Runbookfile", false},
		{"empty path", []string{""}, "", true},
		{"too many args", []string{"a", "b"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs(%q) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseArgs(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestParseArgsHelp(t *testing.T) {
	args := [][]string{
		{"--help"},
		{"-h"},
		{"path/to/other", "--help"},
		{"-h", "a", "b"},
	}

	for _, tt := range args {
		t.Run(strings.Join(tt, " "), func(t *testing.T) {
			got, err := parseArgs(tt)
			if !errors.Is(err, errHelpRequested) {
				t.Fatalf("parseArgs(%q) error = %v, want errHelpRequested", tt, err)
			}
			if got != "" {
				t.Errorf("parseArgs(%q) = %q, want empty path", tt, got)
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
