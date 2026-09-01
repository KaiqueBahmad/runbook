package workdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsure(t *testing.T) {
	t.Run("creates the directory next to the runbookfile", func(t *testing.T) {
		project := t.TempDir()

		dir, err := Ensure(filepath.Join(project, "Runbookfile"))
		if err != nil {
			t.Fatalf("Ensure(): %v", err)
		}
		want := filepath.Join(project, Name)
		if dir != want {
			t.Errorf("Ensure() = %q, want %q", dir, want)
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

		dir := filepath.Join(project, Name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
		file := filepath.Join(dir, "keep")
		if err := os.WriteFile(file, []byte("state\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", file, err)
		}

		if _, err := Ensure(filepath.Join(project, "Runbookfile")); err != nil {
			t.Fatalf("Ensure(): %v", err)
		}
		if _, err := os.Stat(file); err != nil {
			t.Errorf("stat %s: %v", file, err)
		}
	})

	t.Run("existing gitignore is kept", func(t *testing.T) {
		project := t.TempDir()

		dir := filepath.Join(project, Name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
		ignore := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(ignore, []byte("logs/\n"), 0o600); err != nil {
			t.Fatalf("writing %s: %v", ignore, err)
		}

		if _, err := Ensure(filepath.Join(project, "Runbookfile")); err != nil {
			t.Fatalf("Ensure(): %v", err)
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

		path := filepath.Join(project, Name)
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}

		if _, err := Ensure(filepath.Join(project, "Runbookfile")); err == nil {
			t.Fatal("Ensure() error = nil, want an error")
		}
	})
}
