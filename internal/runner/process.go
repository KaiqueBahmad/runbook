//go:build !windows

package runner

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
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

// killGroup sends SIGKILL to every process in the group.
func killGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

// terminateGroup sends SIGTERM to the process group leader.
func terminateGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

// processAlive reports whether the process with the given pid is running.
// It uses the signal 0 check: sending no signal, just asking the kernel
// whether the process is there and reachable.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || isPermError(err)
}

func isPermError(err error) bool {
	return err != nil && (err == syscall.EPERM || err == syscall.EACCES)
}
