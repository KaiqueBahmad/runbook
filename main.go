package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("Hello Fyne")

	label := widget.NewLabel("Hello, World!")
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
