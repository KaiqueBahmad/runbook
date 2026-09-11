//go:build !windows

package runner

import (
	"errors"
	"os/exec"
	"syscall"
)

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
