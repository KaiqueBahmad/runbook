package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	// "fyne.io/fyne/v2"
	// "fyne.io/fyne/v2/app"
	// "fyne.io/fyne/v2/container"
	// "fyne.io/fyne/v2/widget"
)

func main() {
	in, err := parseArgs(os.Args[1:])
	if errors.Is(err, errHelpRequested) {
		fmt.Println(help)
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n%s\n", err, helpHint)
		os.Exit(2)
	}

	// completion runs from wherever a shell starts up, so it never looks at
	// the Runbookfile.
	if in.cmd == cmdCompletion {
		fmt.Print(completionScript(in.rest[0]))
		return
	}

	// broadcast is Runbook talking to itself: start left it holding one end of
	// a pipe, and all it needs is the address to hand what comes down it to.
	if in.cmd == cmdBroadcast {
		if err := broadcast(in.rest[0], os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := checkRunbookfile(in.path); err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
		os.Exit(1)
	}

	entries, err := readRunbookfile(in.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
		os.Exit(1)
	}

	// list, logs and run only read, so they leave no .runbook directory
	if in.cmd == cmdList {
		printNames(os.Stdout, entries, isTerminal(os.Stdout))
		return
	}

	if in.cmd == cmdLogs {
		if err := logs(in, entries, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// The commands that work with what is running forget what has ended since.
	// list is left out: shell completion calls it on every tab, and it is the
	// one command that writes nothing.
	if in.cmd == cmdStatus || in.cmd == cmdStart || in.cmd == cmdStop {
		if err := sweep(in.path); err != nil {
			// Housekeeping must not stand between someone and their process.
			fmt.Fprintf(os.Stderr, "runbook: could not tidy up: %v\n", err)
		}
	}

	if in.cmd == cmdStatus {
		found, err := statusOf(in.path, entries)
		if err != nil {
			fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
			os.Exit(1)
		}
		printStatus(os.Stdout, found, isTerminal(os.Stdout))
		return
	}

	if in.cmd == cmdStart || in.cmd == cmdStop {
		act := start
		if in.cmd == cmdStop {
			act = stop
		}
		if err := act(in, entries, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if in.cmd == cmdRun {
		entry, err := findEntry(entries, in.rest[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
			os.Exit(1)
		}
		code, err := runEntry(entry, filepath.Dir(in.path), os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
			os.Exit(1)
		}
		os.Exit(code)
	}

	mainCommand(os.Stdout, in.path)

	if _, err := ensureRunbookDir(in.path); err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
		os.Exit(1)
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
}
