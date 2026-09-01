package runner

import (
	"runbook/internal/ipc"
	"runbook/internal/state"
)

// Sweep forgets what the commands of a Runbookfile have left behind since they
// ended: the state files of processes that are gone, and the addresses nobody
// is broadcasting at any more.
//
// It is housekeeping, not correctness: every command already treats a missing
// state file and a dead process the same way, and an address with nobody
// behind it is bound over on the next start.
func Sweep(path string) error {
	if err := state.Sweep(path); err != nil {
		return err
	}
	return ipc.Sweep(path)
}
