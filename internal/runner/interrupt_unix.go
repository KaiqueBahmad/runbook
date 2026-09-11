//go:build !windows

package runner

import (
	"os"
	"os/signal"
	"syscall"
)

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
