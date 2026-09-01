package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"runbook/internal/ipc"
	"runbook/internal/runbookfile"
	"runbook/internal/runner"
	"runbook/internal/workdir"
	// "fyne.io/fyne/v2"
	// "fyne.io/fyne/v2/app"
	// "fyne.io/fyne/v2/container"
	// "fyne.io/fyne/v2/widget"
)

// Main carries out what the command line asked for, and gives back the status
// runbook exits with. args is the arguments without the program name.
func Main(args []string) int {
	in, err := parseArgs(args)
	if errors.Is(err, errHelpRequested) {
		fmt.Println(help)
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n%s\n", err, helpHint)
		return 2
	}

	switch in.cmd {
	// completion runs from wherever a shell starts up, so it never looks at
	// the Runbookfile.
	case cmdCompletion:
		fmt.Print(completionScript(in.rest[0]))
		return 0

	// broadcast is Runbook talking to itself: start left it holding one end of
	// a pipe, and all it needs is the address to hand what comes down it to.
	case cmdBroadcast:
		return report(ipc.Broadcast(in.rest[0], os.Stdin))
	}

	if err := runbookfile.Check(in.path); err != nil {
		return report(err)
	}
	entries, err := runbookfile.Read(in.path)
	if err != nil {
		return report(err)
	}

	// The commands that work with what is running forget what has ended since.
	// list is left out: shell completion calls it on every tab, and it is the
	// one command that writes nothing.
	if in.cmd == cmdStatus || in.cmd == cmdStart || in.cmd == cmdStop {
		if err := runner.Sweep(in.path); err != nil {
			// Housekeeping must not stand between someone and their process.
			fmt.Fprintf(os.Stderr, "runbook: could not tidy up: %v\n", err)
		}
	}

	switch in.cmd {
	case cmdList:
		runbookfile.PrintNames(os.Stdout, entries, isTerminal(os.Stdout))
		return 0

	case cmdLogs:
		return report(runner.Logs(in.path, entries, in.rest[0], os.Stdout))

	case cmdStatus:
		found, err := runner.Status(in.path, entries)
		if err != nil {
			return report(err)
		}
		runner.PrintStatus(os.Stdout, found, isTerminal(os.Stdout))
		return 0

	case cmdStart:
		return report(runner.Start(in.path, entries, in.rest[0], os.Stdout))

	case cmdStop:
		return report(runner.Stop(in.path, entries, in.rest[0], os.Stdout))

	case cmdRun:
		code, err := runner.Run(in.path, entries, in.rest[0], os.Stdout, os.Stderr)
		if err != nil {
			return report(err)
		}
		// runbook exits with the status its command exited with.
		return code
	}

	// No command at all opens the panel, which is not there yet.
	mainCommand(os.Stdout, in.path)

	if _, err := workdir.Ensure(in.path); err != nil {
		return report(err)
	}

	// a := app.New()
	// w := a.NewWindow("Runbook")

	// label := widget.NewLabel(fmt.Sprintf("Runbookfile: %s", path))
	// button := widget.NewButton("Click me", func() {
	// 	label.SetText("Button clicked!")
	// })

	// w.SetContent(container.NewVBox(
	// 	label,
	// 	button,
	// ))

	// w.Resize(fyne.NewSize(300, 200))
	// w.ShowAndRun()

	return 0
}

// report says what went wrong, if anything did, and gives back the status to
// exit with.
func report(err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
	return 1
}

// mainCommand stands in for the panel until there is one: by the time it is
// called the Runbookfile has been read, so it says so.
func mainCommand(w io.Writer, path string) {
	fmt.Fprintf(w, "parsed %s successfully\n", path)
}

// isTerminal reports whether f is a terminal, rather than a pipe or a file
// something else will read.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
