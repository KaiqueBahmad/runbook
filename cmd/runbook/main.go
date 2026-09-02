// Runbook is a control panel for the commands a project lists in a
// runbook.yml: run one in this terminal, start one in the background, stop it
// again, and listen to what it writes.
package main

import (
	"os"

	"runbook/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
