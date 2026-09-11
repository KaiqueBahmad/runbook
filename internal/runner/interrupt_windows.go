//go:build windows

package runner

import (
	"os"
	"os/signal"
)

// ignoreInterrupts stops the interrupt signals from ending Runbook itself, and
// returns the function that puts them back. On Windows, there is no SIGTERM,
// so we only handle Ctrl+C (os.Interrupt).
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
