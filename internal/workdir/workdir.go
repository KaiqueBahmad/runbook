// Package workdir is the directory Runbook keeps its own files in, and the
// sweeping of what it leaves behind there.
package workdir

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Name is the directory Runbook creates alongside the runbook.yml to keep its
// own files.
const Name = ".runbook"

// gitignoreAll is the content Runbook puts in the directory's .gitignore. The
// pattern covers the ignore file itself, so nothing in there ever reaches the
// project's git status.
const gitignoreAll = "*\n"

// Path is where Runbook keeps the files of the runbook.yml at path.
func Path(path string) string {
	return filepath.Join(filepath.Dir(path), Name)
}

// Ensure returns the path of Runbook's directory next to the runbook.yml at
// path, creating it if it does not exist yet along with the .gitignore that
// keeps its contents out of the project's repository.
func Ensure(path string) (string, error) {
	dir := Path(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}

	ignore := filepath.Join(dir, ".gitignore")
	switch _, err := os.Stat(ignore); {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.WriteFile(ignore, []byte(gitignoreAll), 0o600); err != nil {
			return "", fmt.Errorf("creating %s: %w", ignore, err)
		}
	case err != nil:
		return "", err
	}
	return dir, nil
}

// SweepUnder clears the files under dir that gone reports are past, and the
// folders left empty once they are, dir itself included. A directory that was
// never there sweeps clean without complaint.
func SweepUnder(dir string, gone func(file string) bool) error {
	empty, err := sweepDir(dir, gone)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if empty {
		return remove(dir)
	}
	return nil
}

// sweepDir clears what gone reports is past under dir, and reports whether
// anything is left in it afterwards.
func sweepDir(dir string, gone func(file string) bool) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	kept := 0
	for _, entry := range entries {
		name := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			empty, err := sweepDir(name, gone)
			if err != nil {
				return false, err
			}
			if !empty {
				kept++
				continue
			}
			if err := remove(name); err != nil {
				return false, err
			}
			continue
		}

		if !gone(name) {
			kept++
			continue
		}
		if err := remove(name); err != nil {
			return false, err
		}
	}
	return kept == 0, nil
}

// remove deletes a file or an empty directory, and is happy if another Runbook
// got there first.
func remove(name string) error {
	if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
