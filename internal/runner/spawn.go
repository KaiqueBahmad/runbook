package runner

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"runbook/internal/runbookfile"
)

// Process is a command Runbook is running itself rather than one it started in
// the background: it belongs to whoever asked for it, writes what it says
// straight to them, and nothing is written down about it. It is what the panel
// runs a command with, where waiting for it would mean waiting on the window.
type Process struct {
	cmd   *exec.Cmd
	ended chan struct{}
	code  int
	err   error
}

// Spawn starts one of a runbook.yml's commands with everything it says going
// to out, and returns as soon as it is going.
//
// The command leads a process group of its own, so stopping it reaches what a
// shell command spawns in turn. It is not a session of its own, unlike a
// started command: this one is meant to end when Runbook does.
func Spawn(path string, entries []runbookfile.Entry, name string, out io.Writer) (*Process, error) {
	entry, err := runbookfile.Find(entries, name)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(shell, "-c", entry.Run)
	cmd.Dir = entryDir(entry, filepath.Dir(path))
	cmd.Env = entryEnv(entry)
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("running %s: %w", entry.Name, err)
	}

	p := &Process{cmd: cmd, ended: make(chan struct{})}
	go func() {
		p.code, p.err = exitStatus(cmd.Wait())
		close(p.ended)
	}()
	return p, nil
}

// PID is the number of the process, which is also the number of its group.
func (p *Process) PID() int {
	return p.cmd.Process.Pid
}

// Ended is closed once the command has ended, so that whoever is waiting for
// it does not have to hold a goroutine of their own to find out.
func (p *Process) Ended() <-chan struct{} {
	return p.ended
}

// Status is how the command ended: the status it exited with, or the error
// that stood in the way of finding out. It is only there once Ended is closed.
func (p *Process) Status() (int, error) {
	return p.code, p.err
}

// Stop ends the command and everything in its group, and reports whether it
// had to be killed outright. It gives it the same grace a stopped command
// gets: a request to end first, and a kill only if that goes unanswered.
func (p *Process) Stop() (bool, error) {
	pid := p.PID()
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		return false, fmt.Errorf("stopping %d: %w", pid, err)
	}

	select {
	case <-p.ended:
		return false, nil
	case <-time.After(grace):
	}

	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		return false, fmt.Errorf("killing %d: %w", pid, err)
	}
	<-p.ended
	return true, nil
}
