package runner

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"runbook/internal/runbookfile"
	"runbook/internal/state"
)

// Running is a command that was started and is still going.
type Running struct {
	Name string
	PID  int
	Up   time.Duration
}

// Status gathers the commands of a runbook.yml that are running now, in the
// order the file lists them. A command that was never started, or whose state
// was left behind by a process that has since gone, is simply not among them.
func Status(path string, entries []runbookfile.Entry) ([]Running, error) {
	var found []Running
	for _, entry := range entries {
		st, err := state.Read(state.File(path, entry.Name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !st.Alive() {
			continue
		}
		found = append(found, Running{Name: entry.Name, PID: st.PID, Up: st.Uptime()})
	}
	return found, nil
}

// PrintStatus writes one running command per line: its name, its process id and
// how long it has been up. Aligned, the three line up in columns; not aligned,
// tabs separate them, for something other than a person to read.
func PrintStatus(w io.Writer, found []Running, align bool) {
	if len(found) == 0 {
		if align {
			fmt.Fprintln(w, "nothing running")
		}
		return
	}

	name, pid := 0, 0
	if align {
		for _, r := range found {
			name = max(name, utf8.RuneCountInString(r.Name))
			pid = max(pid, len(strconv.Itoa(r.PID)))
		}
	}

	for _, r := range found {
		if !align {
			fmt.Fprintf(w, "%s\t%d\t%s\n", r.Name, r.PID, r.Up)
			continue
		}
		gap := strings.Repeat(" ", name-utf8.RuneCountInString(r.Name))
		fmt.Fprintf(w, "%s%s  %*d  %s\n", r.Name, gap, pid, r.PID, r.Up)
	}
}
