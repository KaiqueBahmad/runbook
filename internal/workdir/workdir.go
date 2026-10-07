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
	"strings"
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

// pathFile is the file in the directory of one runbook.yml that says which
// runbook.yml it is. The name of the directory is a fingerprint of the path,
// which cannot be turned back into it.
const pathFile = "path"

// Path is where the files of the runbook.yml at path live. It wants the full
// path of the file, since that is what tells one project from another.
func Path(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding the home directory to keep Runbook's files in: %w", err)
	}
	return filepath.Join(home, Name, key(path)), nil
}

// Ensure is Path, with the directory made if it was not there yet, and the
// path of the runbook.yml written down in it for Recorded to read back.
func Ensure(path string) (string, error) {
	dir, err := Path(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	file := filepath.Join(dir, pathFile)
	if err := os.WriteFile(file, []byte(path+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("writing %s: %w", file, err)
	}
	return dir, nil
}

// All is the directory of every runbook.yml Runbook keeps files for. With
// nothing kept yet there are none, and that is not an error.
func All() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("finding the home directory Runbook keeps its files in: %w", err)
	}
	root := filepath.Join(home, Name)
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(root, entry.Name()))
		}
	}
	return dirs, nil
}

// Recorded is the full path of the runbook.yml whose files are kept in dir, as
// Ensure wrote it down. A directory made by a Runbook from before it did gives
// an error matching fs.ErrNotExist.
func Recorded(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, pathFile))
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
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
