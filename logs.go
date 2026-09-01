package main

import (
	"fmt"
	"io"
)

// logs shows what a started command writes, from now on. It connects to the
// broadcaster the command's output goes to and copies what comes back, until
// the command ends and the broadcaster closes the connection, or until whoever
// asked for it presses Ctrl-C.
//
// There is no history behind it. Runbook writes nothing down, so what the
// command said before this connected was heard by whoever was listening then,
// and by nobody else.
func logs(in invocation, entries []Entry, w io.Writer) error {
	entry, err := findEntry(entries, in.rest[0])
	if err != nil {
		return err
	}

	conn, err := dialIPC(ipcAddr(in.path, entry.Name))
	if err != nil {
		return fmt.Errorf("nothing is broadcasting %s, run 'runbook start %s' first", entry.Name, entry.Name)
	}
	defer conn.Close()

	if _, err := io.Copy(w, conn); err != nil {
		return fmt.Errorf("listening to %s: %w", entry.Name, err)
	}
	return nil
}
