package workdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Run("the files of a project are kept in the home directory", func(t *testing.T) {
		dir, err := Path("/home/someone/project/runbook.yml")
		if err != nil {
			t.Fatalf("Path(): %v", err)
		}
		if !strings.HasPrefix(dir, filepath.Join(home, Name)+string(filepath.Separator)) {
			t.Errorf("Path() = %q, want it under %q", dir, filepath.Join(home, Name))
		}
	})

	t.Run("a project is named after the directory it is in", func(t *testing.T) {
		dir, err := Path("/home/someone/project/runbook.yml")
		if err != nil {
			t.Fatalf("Path(): %v", err)
		}
		if !strings.HasPrefix(filepath.Base(dir), "project-") {
			t.Errorf("Path() = %q, want a directory named after the project", dir)
		}
	})

	t.Run("two projects of the same name do not share it", func(t *testing.T) {
		one, _ := Path("/home/someone/work/api/runbook.yml")
		two, _ := Path("/home/someone/play/api/runbook.yml")
		if one == two {
			t.Errorf("two projects called api come to the same place, %q", one)
		}
	})

	t.Run("two files in one project do not share it either", func(t *testing.T) {
		one, _ := Path("/home/someone/project/runbook.yml")
		two, _ := Path("/home/someone/project/other.yml")
		if one == two {
			t.Errorf("two files of one project come to the same place, %q", one)
		}
	})

	t.Run("the same file always comes to the same place", func(t *testing.T) {
		one, _ := Path("/home/someone/project/runbook.yml")
		two, _ := Path("/home/someone/project/runbook.yml")
		if one != two {
			t.Errorf("Path() = %q and then %q for the one file", one, two)
		}
	})

	t.Run("an address bound under it has room to spare", func(t *testing.T) {
		// A socket address is a path, and the kernel takes about a hundred
		// characters of it. However deep a project sits, and whatever it is
		// called, what Runbook adds to the home directory stays about the
		// same, and what is left is for the command's own name.
		const room = 45

		deep := "/home/someone/" + strings.Repeat("a-long-directory/", 8) + "runbook.yml"
		dir, err := Path(deep)
		if err != nil {
			t.Fatalf("Path(): %v", err)
		}
		if added := len(dir) - len(home); added > room {
			t.Errorf("Path() adds %d characters to the home directory, want at most %d: %q", added, room, dir)
		}
	})
}

func TestEnsure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := "/home/someone/project/runbook.yml"

	t.Run("creates the directory", func(t *testing.T) {
		dir, err := Ensure(path)
		if err != nil {
			t.Fatalf("Ensure(): %v", err)
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	})

	t.Run("an existing directory is kept as it is", func(t *testing.T) {
		dir, err := Ensure(path)
		if err != nil {
			t.Fatalf("Ensure(): %v", err)
		}
		file := filepath.Join(dir, "keep")
		if err := os.WriteFile(file, []byte("state\n"), 0o600); err != nil {
			t.Fatalf("writing %s: %v", file, err)
		}

		if _, err := Ensure(path); err != nil {
			t.Fatalf("Ensure(): %v", err)
		}
		if _, err := os.Stat(file); err != nil {
			t.Errorf("stat %s: %v", file, err)
		}
	})

	t.Run("nothing is left in the project", func(t *testing.T) {
		project := t.TempDir()
		if _, err := Ensure(filepath.Join(project, "runbook.yml")); err != nil {
			t.Fatalf("Ensure(): %v", err)
		}
		left, err := os.ReadDir(project)
		if err != nil {
			t.Fatalf("reading %s: %v", project, err)
		}
		if len(left) != 0 {
			t.Errorf("%d files were put in the project, want it left as it was found", len(left))
		}
	})
}
