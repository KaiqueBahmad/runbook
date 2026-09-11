//go:build windows

// The whole of what Runbook asks of the machine it runs commands on lives in
// this file and in its Unix twin: the shell that runs a command, the way a
// command is put in a group of its own and taken down again, what its exit
// status means, and how Runbook keeps an interrupt from ending itself. The
// rest of the package knows nothing about which machine it is on.

package runner

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"

	"runbook/internal/winapi"
)

// shell is what a command's run line is handed to, and shellFlag is how that
// shell is told to take it from the command line.
const (
	shell     = "cmd.exe"
	shellFlag = "/c"
)

// newProcessGroup is CREATE_NEW_PROCESS_GROUP, which is as close as Windows
// comes to a process group of one command's own: what the command spawns is
// in the group with it, and taskkill /T reaches the lot.
const newProcessGroup = 0x00000200

// setSession puts the command at the head of a group of its own, so that
// taking the command down takes the whole tree with it.
func setSession(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: newProcessGroup}
}

// setProcessGroup puts the command in a process group of its own, so stopping
// it reaches what it spawns. The command is the leader of the group.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: newProcessGroup}
}

// terminateGroup sends a Ctrl+Break event to the process group, which is the
// closest equivalent to SIGTERM. If the process has no console, it is
// terminated outright.
func terminateGroup(pid int) error {
	return taskkill(pid)
}

// killGroup ends every process in the group outright.
func killGroup(pid int) error {
	return taskkill(pid)
}

// taskkill takes the group led by pid down, /T reaching what the command
// spawned in turn. A process that is already gone is not a failure: there is
// nothing left to take down, which is what the caller wanted.
func taskkill(pid int) error {
	if !processAlive(pid) {
		return nil
	}
	err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
	if err != nil && !processAlive(pid) {
		return nil
	}
	return err
}

// processAlive reports whether the process with the given pid is running.
func processAlive(pid int) bool {
	return winapi.Alive(pid)
}

// exitStatus is the status a command exited with. Windows has no signals, so
// there is nothing here of the shell's 128 plus the signal. The error is only
// there when the command could not be waited for at all.
func exitStatus(err error) (int, error) {
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exit):
		if code := exit.ExitCode(); code >= 0 {
			return code, nil
		}
		return 1, nil
	default:
		return 0, err
	}
}

// ignoreInterrupts stops the interrupt signals from ending Runbook itself, and
// returns the function that puts them back. Windows has no SIGTERM, so Ctrl-C
// is the whole of it.
func ignoreInterrupts() func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	go func() {
		for range signals {
		}
	}()
	return func() {
		signal.Stop(signals)
		close(signals)
	}
}
