//go:build !windows

// The whole of what Runbook asks of the machine it runs commands on lives in
// this file and in its Windows twin: the shell that runs a command, the way a
// command is put in a group of its own and taken down again, what its exit
// status means, and how Runbook keeps an interrupt from ending itself. The
// rest of the package knows nothing about which machine it is on.

package runner

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// shell is what a command's run line is handed to, and shellFlag is how that
// shell is told to take it from the command line.
const (
	shell     = "sh"
	shellFlag = "-c"
)

// setSession puts the command in a session of its own, so it leads a process
// group of its own and taking the command down takes the whole tree with it.
func setSession(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// setProcessGroup puts the command in a process group of its own, so stopping
// it reaches what it spawns. The command is the leader of the group.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateGroup asks every process in the group to end.
func terminateGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

// killGroup ends every process in the group outright.
func killGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

// processAlive reports whether the process with the given pid is running.
// It uses the signal 0 check: sending no signal, just asking the kernel
// whether the process is there and reachable.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || isPermError(err)
}

// isPermError is the kernel saying the process is there but not Runbook's to
// signal, which is an answer about the process rather than a failure.
func isPermError(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES)
}

// exitStatus is the status a command exited with, the way a shell reports it: a
// command killed by a signal is 128 plus that signal. The error is only there
// when the command could not be waited for at all.
func exitStatus(err error) (int, error) {
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exit):
		if code := exit.ExitCode(); code >= 0 {
			return code, nil
		}
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal()), nil
		}
		return 1, nil
	default:
		return 0, err
	}
}

// ignoreInterrupts stops the interrupt signals from ending Runbook itself, and
// returns the function that puts them back.
func ignoreInterrupts() func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range signals {
		}
	}()
	return func() {
		signal.Stop(signals)
		close(signals)
	}
}
