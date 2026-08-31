package main

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	path, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n%s\n", err, usage)
		os.Exit(2)
	}

	if err := checkRunbookfile(path); err != nil {
		fmt.Fprintf(os.Stderr, "runbook: %v\n", err)
		os.Exit(1)
	}

	a := app.New()
	w := a.NewWindow("Runbook")

	label := widget.NewLabel(fmt.Sprintf("Runbookfile: %s", path))
	button := widget.NewButton("Click me", func() {
		label.SetText("Button clicked!")
	})

	w.SetContent(container.NewVBox(
		label,
		button,
	))

	w.Resize(fyne.NewSize(300, 200))
	w.ShowAndRun()
}
