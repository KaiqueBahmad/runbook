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

func TestEnsureMetadataFile(t *testing.T) {
	t.Run("creates a file named after the runbookfile", func(t *testing.T) {
		project := t.TempDir()
		dir := filepath.Join(project, runbookDirName)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}

		metadata, err := ensureMetadataFile(dir, filepath.Join(project, "other-runbookfile"))
		if err != nil {
			t.Fatalf("ensureMetadataFile(): %v", err)
		}
		want := filepath.Join(dir, "other-runbookfile.metadata")
		if metadata != want {
			t.Errorf("ensureMetadataFile() = %q, want %q", metadata, want)
		}
		got, err := os.ReadFile(metadata)
		if err != nil {
			t.Fatalf("reading %s: %v", metadata, err)
		}
		if len(got) != 0 {
			t.Errorf("%s = %q, want it empty", metadata, got)
		}
	})

	t.Run("existing metadata is kept", func(t *testing.T) {
		project := t.TempDir()
		dir := filepath.Join(project, runbookDirName)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
		metadata := filepath.Join(dir, "Runbookfile"+metadataExt)
		if err := os.WriteFile(metadata, []byte("api: stopped\n"), 0o600); err != nil {
			t.Fatalf("writing %s: %v", metadata, err)
		}

		if _, err := ensureMetadataFile(dir, filepath.Join(project, "Runbookfile")); err != nil {
			t.Fatalf("ensureMetadataFile(): %v", err)
		}
		got, err := os.ReadFile(metadata)
		if err != nil {
			t.Fatalf("reading %s: %v", metadata, err)
		}
		if string(got) != "api: stopped\n" {
			t.Errorf("%s = %q, want it left alone", metadata, got)
		}
	})

	t.Run("missing directory", func(t *testing.T) {
		project := t.TempDir()

		if _, err := ensureMetadataFile(filepath.Join(project, runbookDirName), filepath.Join(project, "Runbookfile")); err == nil {
			t.Fatal("ensureMetadataFile() error = nil, want an error")
		}
	})
}
