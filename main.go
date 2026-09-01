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

	if err := checkRunbookfile(in.path); err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
		os.Exit(1)
	}

	entries, err := readRunbookfile(in.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
		os.Exit(1)
	}

	// list and run only read, so they leave no .runbook directory behind.
	if in.cmd == cmdList {
		printNames(os.Stdout, entries, isTerminal(os.Stdout))
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

	printEntries(os.Stdout, entries)

	dir, err := ensureRunbookDir(in.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
		os.Exit(1)
	}

	if _, err := ensureMetadataFile(dir, in.path); err != nil {
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
