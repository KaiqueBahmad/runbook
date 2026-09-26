package runner

import (
	"fmt"
	"io"
	"path/filepath"

	"runbook/internal/runbookfile"
)

// Inspect writes what the named command runs, the way it could be typed into a
// shell: its run, behind a cd into its dir when it has one. The dir is written
// out in full, since the one in the file is measured from the runbook.yml
// rather than from wherever this is read. A command with no dir runs where it
// is typed, so it gets no cd at all.
func Inspect(path string, entries []runbookfile.Entry, name string, w io.Writer) error {
	entry, err := runbookfile.Find(entries, name)
	if err != nil {
		return err
	}
	if dir := entryDir(entry, filepath.Dir(path)); dir != "" {
		fmt.Fprintf(w, "%s && ", cdTo(dir))
	}
	fmt.Fprintln(w, entry.Run)
	return nil
}
