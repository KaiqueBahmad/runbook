package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// grace is how long a stopped command has to end on its own after SIGTERM,
// before Runbook stops asking and kills it.
const grace = 5 * time.Second

// startEntry starts a command in the background and records its process id, so
// that a stop from another terminal can find it again. The command gets a
// process group of its own: it is what a shell command's own children join, so
// stopping it can reach the whole tree, and it keeps the command out of the
// terminal's foreground group, so Ctrl-C here does not reach it.
//
// Nothing reads what the command writes, so its output goes to the null
// device: leaving it on the terminal would have it turn up in whatever the
// person running Runbook does next.
func startEntry(entry Entry, base, state string) (int, error) {
	if st, err := readState(state); err == nil && st.alive() {
		return 0, fmt.Errorf("%s is already running (pid %d)", entry.Name, st.PID)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}

	cmd := exec.Command(shell, "-c", entry.Run)
	cmd.Dir = entryDir(entry, base)
	cmd.Env = entryEnv(entry)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("starting %s: %w", entry.Name, err)
	}
	pid := cmd.Process.Pid

	// Setpgid makes the command the leader of its own group, so the group has
	// the same number as the process.
	boot, _ := processBoot(pid)
	if err := writeState(state, stateOf(pid, boot)); err != nil {
		// Nothing knows about the process now, so do not leave it behind.
		syscall.Kill(-pid, syscall.SIGKILL)
		return 0, err
	}
	return pid, nil
}

func stateOf(pid int, boot string) state {
	return state{PID: pid, Boot: boot, Since: time.Now().Unix()}
}

// stopEntry ends a started command and forgets it, and reports whether it had
// to be killed outright. The signals go to the whole process group, so a
// command that is a shell script takes what it spawned down with it.
func stopEntry(state string, wait time.Duration) (bool, error) {
	st, err := readState(state)
	if errors.Is(err, fs.ErrNotExist) {
		return false, errors.New("not running")
	}
	if err != nil {
		return false, err
	}
	if !st.alive() {
		// The process is long gone; only the file was left behind.
		return false, errors.Join(errors.New("not running"), os.Remove(state))
	}

	if err := syscall.Kill(-st.group(), syscall.SIGTERM); err != nil {
		return false, fmt.Errorf("stopping %d: %w", st.PID, err)
	}
	killed := false
	if !waitGone(st, wait) {
		if err := syscall.Kill(-st.group(), syscall.SIGKILL); err != nil {
			return false, fmt.Errorf("killing %d: %w", st.PID, err)
		}
		killed = true
		waitGone(st, wait)
	}
	if err := os.Remove(state); err != nil {
		return killed, err
	}
	return killed, nil
}

// waitGone reports whether the command ended within the time given.
func waitGone(st state, wait time.Duration) bool {
	const step = 20 * time.Millisecond
	for waited := time.Duration(0); waited < wait; waited += step {
		if !st.alive() {
			return true
		}
		time.Sleep(step)
	}
	return !st.alive()
}

// start and stop are what the commands of the same name do: find the entry,
// make sure Runbook has somewhere to keep its files, and report what happened.
func start(in invocation, entries []Entry, w io.Writer) error {
	entry, err := findEntry(entries, in.rest[0])
	if err != nil {
		return err
	}
	if _, err := ensureRunbookDir(in.path); err != nil {
		return err
	}

	pid, err := startEntry(entry, filepath.Dir(in.path), stateFile(in.path, entry.Name))
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "started %s (pid %d)\n", entry.Name, pid)
	return nil
}

func stop(in invocation, entries []Entry, w io.Writer) error {
	entry, err := findEntry(entries, in.rest[0])
	if err != nil {
		return err
	}

	killed, err := stopEntry(stateFile(in.path, entry.Name), grace)
	if err != nil {
		return fmt.Errorf("%s: %w", entry.Name, err)
	}
	if killed {
		fmt.Fprintf(w, "killed %s, it ignored the request to stop\n", entry.Name)
		return nil
	}
	fmt.Fprintf(w, "stopped %s\n", entry.Name)
	return nil
}
