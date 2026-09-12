// Package runner carries out what a runbook.yml lists: running a command in
// this terminal, starting one in the background and stopping it again, saying
// which are running, and listening to what a started one writes.
package runner

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"runbook/internal/runbookfile"
)

// grace is how long a stopped command has to end on its own after SIGTERM,
// before Runbook stops asking and kills it. Every way of ending a command
// gives it the same one, whether it was started in the background or is
// Runbook's own to wait on.
const grace = 5 * time.Second

// entryDir is the directory a command runs in. base is the directory the
// runbook.yml lives in, which a relative dir is measured from.
func entryDir(entry runbookfile.Entry, base string) string {
	switch {
	case entry.Dir == "":
		return base
	case filepath.IsAbs(entry.Dir):
		return entry.Dir
	default:
		return filepath.Join(base, entry.Dir)
	}
}

// entryEnv is the environment a command runs with: the one Runbook was started
// with, plus the command's own variables. It is nil when the command adds
// none, which leaves the environment untouched.
func entryEnv(entry runbookfile.Entry) []string {
	if len(entry.Env) == 0 {
		return nil
	}
	env := os.Environ()
	for _, name := range slices.Sorted(maps.Keys(entry.Env)) {
		env = append(env, name+"="+entry.Env[name])
	}
	return env
}
