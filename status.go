package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// running is a command that was started and is still going.
type running struct {
	name string
	pid  int
	up   time.Duration
}

// statusOf gathers the commands of a Runbookfile that are running now, in the
// order the file lists them. A command that was never started, or whose state
// was left behind by a process that has since gone, is simply not among them.
func statusOf(path string, entries []Entry) ([]running, error) {
	var found []running
	for _, entry := range entries {
		st, err := readState(stateFile(path, entry.Name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !st.alive() {
			continue
		}
		found = append(found, running{name: entry.Name, pid: st.PID, up: st.since()})
	}
	return found, nil
}

// printStatus writes one running command per line: its name, its process id and
// how long it has been up. Aligned, the three line up in columns; not aligned,
// tabs separate them, for something other than a person to read.
func printStatus(w io.Writer, found []running, align bool) {
	if len(found) == 0 {
		if align {
			fmt.Fprintln(w, "nothing running")
		}
		return
	}

	name, pid := 0, 0
	if align {
		for _, r := range found {
			name = max(name, utf8.RuneCountInString(r.name))
			pid = max(pid, len(strconv.Itoa(r.pid)))
		}
	}

	for _, r := range found {
		if !align {
			fmt.Fprintf(w, "%s\t%d\t%s\n", r.name, r.pid, r.up)
			continue
		}
		gap := strings.Repeat(" ", name-utf8.RuneCountInString(r.name))
		fmt.Fprintf(w, "%s%s  %*d  %s\n", r.name, gap, pid, r.pid, r.up)
	}
}
