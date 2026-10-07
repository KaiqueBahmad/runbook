package runner

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"runbook/internal/runbookfile"
	"runbook/internal/state"
	"runbook/internal/workdir"
)

// RunningIn is a command that is running for some runbook.yml, not
// necessarily the one at hand.
type RunningIn struct {
	Running

	// File is the full path of the runbook.yml the command belongs to. It is
	// empty when the command was started by a Runbook from before that was
	// written down, and Work is all there is to tell it by.
	File string
	Work string
}

// StatusAll gathers every command that is running, whichever runbook.yml it
// belongs to, a runbook.yml at a time and in the order of their paths. It
// reads what start remembered rather than the runbook.yml, which may have
// changed or gone since.
func StatusAll() ([]RunningIn, error) {
	dirs, err := workdir.All()
	if err != nil {
		return nil, err
	}

	var found []RunningIn
	for _, work := range dirs {
		file, err := workdir.Recorded(work)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		err = state.Each(work, func(name string, st state.State) {
			if st.Alive() {
				found = append(found, RunningIn{
					Running: Running{Name: name, PID: st.PID, Up: st.Uptime()},
					File:    file,
					Work:    work,
				})
			}
		})
		if err != nil {
			return nil, err
		}
	}

	// Each keeps the names of one runbook.yml in order, and a stable sort
	// keeps them that way.
	slices.SortStableFunc(found, func(a, b RunningIn) int {
		return strings.Compare(a.File+a.Work, b.File+b.Work)
	})
	return found, nil
}

// PrintStatusAll writes one running command per line, as PrintStatus does,
// behind where its runbook.yml is. Aligned, that is the project's directory,
// with home as ~, and the whole path only for a file not called runbook.yml;
// not aligned, it is the full path of the file, tab-separated like the rest.
func PrintStatusAll(w io.Writer, found []RunningIn, align bool, home string) {
	if len(found) == 0 {
		if align {
			fmt.Fprintln(w, "nothing running")
		}
		return
	}

	if !align {
		for _, r := range found {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", r.File, r.Name, r.PID, r.Up)
		}
		return
	}

	where := make([]string, len(found))
	at, name, pid := 0, 0, 0
	for i, r := range found {
		where[i] = project(r, home)
		at = max(at, utf8.RuneCountInString(where[i]))
		name = max(name, utf8.RuneCountInString(r.Name))
		pid = max(pid, len(strconv.Itoa(r.PID)))
	}
	for i, r := range found {
		fmt.Fprintf(w, "%s  %s  %*d  %s\n", pad(where[i], at), pad(r.Name, name), pid, r.PID, r.Up)
	}
}

// project is where a running command's runbook.yml is, for a person: its
// directory, or the file itself when it has another name, so that two files
// in one directory are told apart. One whose path was never written down goes
// by the directory Runbook keeps for it.
func project(r RunningIn, home string) string {
	if r.File == "" {
		return filepath.Base(r.Work)
	}
	where := r.File
	if filepath.Base(where) == runbookfile.Name {
		where = filepath.Dir(where)
	}
	if home == "" {
		return where
	}
	if where == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(where, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return where
}

// pad fills s out with spaces to width characters.
func pad(s string, width int) string {
	return s + strings.Repeat(" ", width-utf8.RuneCountInString(s))
}
