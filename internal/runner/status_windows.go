//go:build windows

package runner

import (
	"errors"
	"os/exec"
)

// exitStatus is the status a command exited with. On Windows, signal-based
// exit codes are not applicable, so we just return the exit code directly.
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
