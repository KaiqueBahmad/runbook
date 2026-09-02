// Package gui is the control panel: the commands of a runbook.yml on the left,
// and what one of them is saying on the right.
package gui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"runbook/internal/ipc"
	"runbook/internal/runbookfile"
	"runbook/internal/runner"
	"runbook/internal/state"
)

// draw is how often the window catches up with what the commands are saying.
// It is the output that arrives on its own; what a command is doing is only
// ever looked at when someone asks.
const draw = 200 * time.Millisecond

// how a command stands, which is the whole of what says which buttons a person
// can press on it.
type how int

const (
	idle    how = iota // nothing of Runbook's is running it
	started            // running in the background, where a stop can find it
	running            // running from this window, and ending with it
)

// command is what the window knows about one of the runbook.yml's commands.
type command struct {
	entry runbookfile.Entry
	out   *tail // the last of what it has said

	how  how
	pid  int
	proc *runner.Process // what is running it, while this window is

	// heard is whether the window is connected to the broadcaster of a started
	// command. It is the one thing here the listening goroutine sets itself,
	// rather than coming back to the window for it, so that letting go of a
	// command that has ended costs nothing.
	heard atomic.Bool
}

// mark is how a command is shown in the list, beside its name.
func (c *command) mark() string {
	switch c.how {
	case started:
		return fmt.Sprintf("started (%d)", c.pid)
	case running:
		return fmt.Sprintf("running (%d)", c.pid)
	default:
		return ""
	}
}

// panel is the window: the commands of one runbook.yml, and what it knows
// about each of them.
//
// Everything in here is touched from the goroutine that draws the window and
// from nowhere else. What listens to a command writes to its tail, which is
// safe to, and comes back through fyne.Do for the rest.
type panel struct {
	path    string
	entries []runbookfile.Entry
	byName  map[string]*command
	folders *folders

	win    fyne.Window
	tree   *widget.Tree
	count  *widget.Label
	head   *widget.Label
	output *widget.TextGrid
	scroll *container.Scroll

	run, start, stop, logs *widget.Button

	// doing is the work the window has out at the moment. Nothing in the
	// window waits for it — that is the point of putting it out of the way —
	// but the tests do, to know when an action has come back before they look
	// at what it did.
	doing sync.WaitGroup

	picked string // the node selected in the list, which the buttons act on
	shown  string // the command whose output is on the right
	busy   bool   // an action is under way, and nothing else can be asked for
}

// Open shows the panel for a runbook.yml, and returns once it is closed.
func Open(path string, entries []runbookfile.Entry) error {
	p := newPanel(path, entries)

	a := app.New()
	a.Settings().SetTheme(folderIcons{theme.Current()})

	p.win = a.NewWindow("Runbook — " + path)
	p.win.SetContent(p.content())
	p.win.Resize(fyne.NewSize(1000, 640))

	// What is running now, and what there is to listen to.
	p.refresh()

	done := make(chan struct{})
	p.win.SetOnClosed(func() { close(done) })
	go p.follow(done)

	p.win.ShowAndRun()

	// The commands this window ran are its own, so they go with it.
	for _, c := range p.byName {
		if c.proc != nil {
			c.proc.Stop()
		}
	}
	return nil
}

func newPanel(path string, entries []runbookfile.Entry) *panel {
	p := &panel{
		path:    path,
		entries: entries,
		byName:  make(map[string]*command, len(entries)),
		folders: newFolders(entries),
	}
	for _, entry := range entries {
		p.byName[entry.Name] = &command{entry: entry, out: &tail{}}
	}
	return p
}

// content lays the window out: the commands on the left with what can be done
// to the one that is selected under them, and its output on the right.
func (p *panel) content() fyne.CanvasObject {
	p.tree = widget.NewTree(p.folders.childrenOf, p.folders.isBranch, newNode, p.fillNode)
	p.tree.OnSelected = p.pick

	p.run = widget.NewButtonWithIcon("Run", theme.MediaPlayIcon(), p.doRun)
	p.start = widget.NewButtonWithIcon("Start", theme.MediaFastForwardIcon(), p.doStart)
	p.stop = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), p.doStop)
	p.logs = widget.NewButtonWithIcon("Logs", theme.DocumentIcon(), p.doLogs)
	p.buttons()

	left := container.NewBorder(nil, container.NewGridWithColumns(2, p.run, p.start, p.stop, p.logs), nil, nil, p.tree)

	p.head = widget.NewLabel("")
	p.head.TextStyle = fyne.TextStyle{Bold: true}
	p.output = widget.NewTextGrid()
	p.scroll = container.NewScroll(p.output)
	right := container.NewBorder(p.head, nil, nil, nil, p.scroll)

	split := container.NewHSplit(left, right)
	split.Offset = 0.3

	return container.NewBorder(p.bar(), nil, nil, nil, split)
}

// bar is the top of the window: which runbook.yml it is working on, how much
// of it is running, and the button that goes and looks again.
func (p *panel) bar() fyne.CanvasObject {
	where := widget.NewLabel(shorten(p.path))
	where.TextStyle = fyne.TextStyle{Bold: true}

	// The state of the commands is what someone asked for last, so there is a
	// button to ask again rather than a window that goes and looks by itself.
	// What that button is for is the count beside it.
	p.count = widget.NewLabel(p.tally())
	p.count.TextStyle = fyne.TextStyle{Italic: true}
	refresh := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), p.refresh)

	bar := container.NewBorder(nil, nil,
		container.NewHBox(widget.NewIcon(theme.FileIcon()), where),
		container.NewHBox(p.count, refresh),
	)
	return container.NewVBox(bar, widget.NewSeparator())
}

// tally is how much of the runbook.yml is running, for the top of the window.
func (p *panel) tally() string {
	going := 0
	for _, c := range p.byName {
		if c.how != idle {
			going++
		}
	}
	switch going {
	case 0:
		return "nothing running"
	case 1:
		return "1 of " + count(len(p.entries)) + " running"
	default:
		return fmt.Sprintf("%d of %s running", going, count(len(p.entries)))
	}
}

// count is a number of commands, said the way a person would.
func count(n int) string {
	if n == 1 {
		return "1 command"
	}
	return fmt.Sprintf("%d commands", n)
}

// shorten is a path as the person who typed it would write it, with their home
// directory as the ~ they call it.
func shorten(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || !strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return path
	}
	return "~" + path[len(home):]
}

// newNode and fillNode are one line of the list: the name of a command or of a
// folder, and where the name is a command's, how it stands.
func newNode(bool) fyne.CanvasObject {
	return container.NewHBox(widget.NewLabel(""), layout.NewSpacer(), widget.NewLabel(""))
}

func (p *panel) fillNode(node widget.TreeNodeID, _ bool, o fyne.CanvasObject) {
	line := o.(*fyne.Container)
	name := line.Objects[0].(*widget.Label)
	mark := line.Objects[2].(*widget.Label)

	name.SetText(label(node))
	if c := p.byName[node]; c != nil {
		mark.SetText(c.mark())
		return
	}
	mark.SetText("")
}

// pick takes what was tapped in the list. A folder is not a command and cannot
// be picked: tapping one opens or closes it, and leaves the command that was
// picked where it was.
func (p *panel) pick(node widget.TreeNodeID) {
	if p.byName[node] != nil {
		p.picked = node
		p.buttons()
		return
	}

	p.tree.Unselect(node)
	p.tree.ToggleBranch(node)
	// Selecting a node scrolls it into view, opening every folder above it on
	// the way, which would undo the very tap that closed one. So the mark goes
	// back on the picked command only where the tap has left it on screen; it
	// is the command the buttons act on either way.
	if p.picked != "" && p.onScreen(p.picked) {
		p.tree.Select(p.picked)
	}
}

// onScreen reports whether a node is in view, which it is when every folder
// above it is open.
func (p *panel) onScreen(node string) bool {
	for cut := strings.LastIndex(node, "/"); cut >= 0; cut = strings.LastIndex(node, "/") {
		node = node[:cut]
		if !p.tree.IsBranchOpen(node) {
			return false
		}
	}
	return true
}

// selection is the command the buttons act on, and is nothing when nothing has
// been picked yet.
func (p *panel) selection() *command {
	return p.byName[p.picked]
}

// buttons is which of them the selected command can be given. A command that
// is not running cannot be stopped, one that is cannot be started again, and
// one that has said nothing has no output to show.
func (p *panel) buttons() {
	c := p.selection()
	if c == nil || p.busy {
		for _, b := range []*widget.Button{p.run, p.start, p.stop, p.logs} {
			b.Disable()
		}
		return
	}
	able(p.run, c.how == idle)
	able(p.start, c.how == idle)
	able(p.stop, c.how != idle)
	able(p.logs, c.how != idle || !c.out.empty())
}

func able(b *widget.Button, on bool) {
	if on {
		b.Enable()
		return
	}
	b.Disable()
}

// doRun runs the selected command from this window, where it is nobody else's
// to stop and ends when the window does.
func (p *panel) doRun() {
	c := p.selection()
	if c == nil {
		return
	}
	p.show(c.entry.Name)

	proc, err := runner.Spawn(p.path, p.entries, c.entry.Name, c.out)
	if err != nil {
		dialog.ShowError(err, p.win)
		return
	}
	c.how, c.proc, c.pid = running, proc, proc.PID()
	c.out.say("running %s (pid %d)", c.entry.Name, proc.PID())
	p.redraw()

	p.doing.Add(1)
	go func() {
		defer p.doing.Done()

		<-proc.Ended()
		code, err := proc.Status()
		// A command this window is running is its own to know about, so how it
		// ended is said here and now rather than waiting to be asked for.
		fyne.Do(func() {
			if c.proc != proc {
				return
			}
			c.how, c.proc, c.pid = idle, nil, 0
			if err != nil {
				c.out.say("%v", err)
			} else {
				c.out.say("%s ended with status %d", c.entry.Name, code)
			}
			p.redraw()
		})
	}()
}

// doStart starts the selected command in the background, where it outlives the
// window and a stop from anywhere can find it.
func (p *panel) doStart() {
	c := p.selection()
	if c == nil {
		return
	}
	p.show(c.entry.Name)
	p.work(
		func() error { return runner.Start(p.path, p.entries, c.entry.Name, c.out) },
		func() { p.look(c); p.listen(c) },
	)
}

// doStop ends the selected command, whichever way it was set going.
func (p *panel) doStop() {
	c := p.selection()
	if c == nil {
		return
	}
	if proc := c.proc; proc != nil {
		p.work(
			func() error {
				killed, err := proc.Stop()
				if err == nil && killed {
					c.out.say("killed %s, it ignored the request to stop", c.entry.Name)
				}
				return err
			},
			func() { c.how, c.proc, c.pid = idle, nil, 0 },
		)
		return
	}
	p.work(
		func() error { return runner.Stop(p.path, p.entries, c.entry.Name, c.out) },
		func() { c.how, c.pid = idle, 0 },
	)
}

// doLogs puts the output of the selected command on the right.
func (p *panel) doLogs() {
	if c := p.selection(); c != nil {
		p.show(c.entry.Name)
	}
}

// work does something that takes a moment, away from the window so that it
// keeps drawing, and comes back to it with what happened. Nothing else can be
// asked for while it is under way: starting a command twice over, because the
// first start had not finished, is not something to leave open.
func (p *panel) work(do func() error, done func()) {
	p.busy = true
	p.buttons()

	p.doing.Add(1)
	go func() {
		defer p.doing.Done()

		err := do()
		fyne.Do(func() {
			p.busy = false
			if err != nil {
				dialog.ShowError(err, p.win)
			} else {
				done()
			}
			p.redraw()
		})
	}()
}

// refresh is the one thing that goes and looks: what the state files say is
// running now, and which broadcasters there are to hear. Nothing else in the
// window finds out on its own, so what is on screen is what was asked for.
func (p *panel) refresh() {
	// Housekeeping must not stand between someone and their commands, so a
	// sweep that will not go through is left where it is.
	runner.Sweep(p.path)

	for _, c := range p.byName {
		p.look(c)
		p.listen(c)
	}
	p.redraw()
}

// look is what the state file says about one command. A command this window is
// running itself is left alone: nothing was written down about it, and the
// window knows better than the directory does.
func (p *panel) look(c *command) {
	if c.proc != nil {
		return
	}
	st, err := state.Read(state.File(p.path, c.entry.Name))
	if err != nil || !st.Alive() {
		c.how, c.pid = idle, 0
		return
	}
	c.how, c.pid = started, st.PID
}

// listen hears what a started command says from now on, and keeps the last of
// it. There is no history behind a broadcaster: what was said before this
// connected was heard by whoever was listening then, and by nobody else.
func (p *panel) listen(c *command) {
	if c.heard.Load() {
		return
	}
	conn, err := ipc.Dial(ipc.Addr(p.path, c.entry.Name))
	if err != nil {
		return // nothing is broadcasting it
	}
	c.heard.Store(true)

	go func() {
		defer conn.Close()
		// The copy ends when the command does, or when its broadcaster does.
		// Either way there is an address to connect to again once someone asks
		// for a refresh.
		defer c.heard.Store(false)
		io.Copy(c.out, conn)
	}()
}

// show puts one command's output on the right.
func (p *panel) show(name string) {
	p.shown = name
	p.drawOutput()
}

// redraw is the window catching up with what has changed: the marks in the
// list, which buttons can be pressed, and the output on the right.
func (p *panel) redraw() {
	p.tree.Refresh()
	p.buttons()
	p.count.SetText(p.tally())
	p.drawOutput()
}

func (p *panel) drawOutput() {
	c := p.byName[p.shown]
	if c == nil {
		p.head.SetText("")
		p.output.SetText("")
		return
	}
	p.head.SetText(p.shown + "  " + c.mark())
	p.output.SetText(c.out.text())
	p.scroll.ScrollToBottom()
}

// follow keeps the output on the right up with what is coming in. It is the
// only thing the window does of its own accord, and it touches nothing but the
// output: what the commands are doing waits to be asked for.
func (p *panel) follow(done <-chan struct{}) {
	tick := time.NewTicker(draw)
	defer tick.Stop()

	for {
		select {
		case <-done:
			return
		case <-tick.C:
			fyne.Do(func() {
				if c := p.byName[p.shown]; c != nil && c.out.fresh() {
					p.drawOutput()
				}
			})
		}
	}
}
