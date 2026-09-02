// Package workdir is where Runbook keeps its own files: one directory per
// runbook.yml, all of them together in the home directory of whoever is
// running it, and the sweeping of what is left behind in them.
package workdir

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Name is the directory Runbook keeps everything it knows in, in the home
// directory of whoever is running it. A project is left exactly as it was
// found: what Runbook writes down is its own business, and it has no place in
// somebody else's repository.
const Name = ".runbook"

// maxName is as much of a project's name as its directory carries. The rest is
// what tells two projects apart anyway, and an address that a socket is bound
// to has only so many characters to give.
const maxName = 16

// Path is where the files of the runbook.yml at path live. It wants the full
// path of the file, since that is what tells one project from another.
func Path(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding the home directory to keep Runbook's files in: %w", err)
	}
	return filepath.Join(home, Name, key(path)), nil
}

// Ensure is Path, with the directory made if it was not there yet.
func Ensure(path string) (string, error) {
	dir, err := Path(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	return dir, nil
}

// key names the directory of one runbook.yml: what the project is called, so
// that someone looking through them can tell which is which, and a fingerprint
// of the whole path, so that two projects of the same name, or two files in
// one project, never come to the same place.
func key(path string) string {
	sum := sha256.Sum256([]byte(path))
	return name(path) + "-" + hex.EncodeToString(sum[:8])
}

// name is what to call the project a runbook.yml belongs to: the directory it
// sits in, kept short, and left out altogether where that is nothing to name a
// directory after.
func name(path string) string {
	dir := filepath.Base(filepath.Dir(path))
	switch dir {
	case ".", "..", string(filepath.Separator):
		return "runbook"
	}
	if runes := []rune(dir); len(runes) > maxName {
		dir = string(runes[:maxName])
	}
	return dir
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
