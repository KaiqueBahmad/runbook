package gui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"runbook/internal/ipc"
	"runbook/internal/runbookfile"
	"runbook/internal/runner"
	"runbook/internal/state"
)

// TestMain sends the test binary to broadcast when it is started as one: the
// panel starts commands the way the command line does, with another copy of
// runbook carrying the output, and under test the copy at hand is this binary.
func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == runner.BroadcastCommand {
		if err := ipc.Broadcast(os.Args[2], os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// testPanel opens a panel on a runbook.yml written in a directory of its own,
// laid out but never shown: everything the buttons do happens without one.
func testPanel(t *testing.T, file string) *panel {
	t.Helper()

	test.NewTempApp(t)

	path := filepath.Join(t.TempDir(), "runbook.yml")
	if err := os.WriteFile(path, []byte(file), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	entries, err := runbookfile.Read(path)
	if err != nil {
		t.Fatalf("runbookfile.Read(): %v", err)
	}

	p := newPanel(path, entries)
	p.win = test.NewTempWindow(t, p.content())
	return p
}

// wants is which buttons the window should be offering.
func wants(t *testing.T, p *panel, run, start, stop, logs bool) {
	t.Helper()

	for _, b := range []struct {
		name string
		on   bool
		of   *widget.Button
	}{
		{"run", run, p.run},
		{"start", start, p.start},
		{"stop", stop, p.stop},
		{"logs", logs, p.logs},
	} {
		if b.of.Disabled() == b.on {
			t.Errorf("%s is disabled = %v, want %v", b.name, b.of.Disabled(), !b.on)
		}
	}
}

// shown is every node the list puts on screen, walked the way the tree widget
// walks it: from the root, and down into a node only where the list says the
// node is a folder.
func shown(tree *widget.Tree, node widget.TreeNodeID) []string {
	if !tree.IsBranch(node) {
		return []string{node}
	}
	var found []string
	for _, child := range tree.ChildUIDs(node) {
		found = append(found, shown(tree, child)...)
	}
	return found
}

// TestList is that the window shows the runbook.yml, all of it: every command
// is there from the moment it opens, whether or not anything is running.
func TestList(t *testing.T) {
	p := testPanel(t, `services/api:
  run: true

services/web:
  run: true

lint:
  run: true
`)

	want := []string{"services/api", "services/web", "lint"}
	if got := shown(p.tree, p.tree.Root); !slices.Equal(got, want) {
		t.Errorf("the list shows %v, want every command of the file: %v", got, want)
	}
}

// TestFoldersStartClosed is that a window opens on the folders of a project
// rather than on every command in it.
func TestFoldersStartClosed(t *testing.T) {
	p := testPanel(t, "services/api:\n  run: true\n\nlint:\n  run: true\n")

	if p.tree.IsBranchOpen("services") {
		t.Error("the folders are open before anyone has opened one")
	}
	if !p.onScreen("lint") {
		t.Error("a command that is in no folder is out of sight")
	}
	if p.onScreen("services/api") {
		t.Error("a command inside a folder is in sight while the folder is shut")
	}
}

func TestButtons(t *testing.T) {
	t.Run("nothing is selected", func(t *testing.T) {
		p := testPanel(t, "api:\n  run: sleep 30\n")
		wants(t, p, false, false, false, false)
	})

	t.Run("a folder is not a command", func(t *testing.T) {
		p := testPanel(t, "web/one:\n  run: true\n")
		p.tree.Select("web")
		wants(t, p, false, false, false, false)
	})

	t.Run("a folder cannot be picked", func(t *testing.T) {
		p := testPanel(t, "web/one:\n  run: true\n\nweb/two:\n  run: true\n")
		p.tree.Select("web/one")

		// Tapping a folder is what opens and closes it, and that is all it is.
		open := p.tree.IsBranchOpen("web")
		p.tree.Select("web")

		if p.picked != "web/one" {
			t.Errorf("tapping a folder left %q picked, want the command to have stayed", p.picked)
		}
		if p.tree.IsBranchOpen("web") == open {
			t.Error("tapping a folder neither opened nor closed it")
		}
		// The buttons still act on the command, which is what is picked.
		wants(t, p, true, true, false, false)
	})

	t.Run("closing a folder leaves its command picked", func(t *testing.T) {
		p := testPanel(t, "web/one:\n  run: true\n")
		p.tree.Select("web/one")

		p.tree.Select("web") // closes it, and the command goes out of view
		if p.tree.IsBranchOpen("web") {
			t.Fatal("the folder is still open")
		}
		if p.picked != "web/one" {
			t.Errorf("%q is picked, want the command that went out of view", p.picked)
		}

		p.tree.Select("web") // and open it again
		if !p.tree.IsBranchOpen("web") {
			t.Fatal("the folder did not open again")
		}
		if p.picked != "web/one" {
			t.Errorf("%q is picked, want the command to have come back with it", p.picked)
		}
	})

	t.Run("a command that is not running", func(t *testing.T) {
		p := testPanel(t, "api:\n  run: sleep 30\n")
		p.tree.Select("api")
		// It can be set going either way, there is nothing to stop, and it has
		// said nothing there would be to show.
		wants(t, p, true, true, false, false)
	})

	t.Run("what a command said outlives it", func(t *testing.T) {
		p := testPanel(t, "api:\n  run: sleep 30\n")
		fmt.Fprintln(p.byName["api"].out, "it said this before it ended")

		p.tree.Select("api")
		wants(t, p, true, true, false, true)
	})

	t.Run("a command running in the background", func(t *testing.T) {
		p := testPanel(t, "api:\n  run: sleep 30\n")
		c := p.byName["api"]
		c.how, c.pid = started, 1234

		p.tree.Select("api")
		// Started twice over is not a thing, and there is something to stop.
		wants(t, p, false, false, true, true)
	})

	t.Run("a command this window is running", func(t *testing.T) {
		p := testPanel(t, "api:\n  run: sleep 30\n")
		c := p.byName["api"]
		c.how, c.pid = running, 1234

		p.tree.Select("api")
		wants(t, p, false, false, true, true)
	})

	t.Run("while Runbook is busy with the last thing asked of it", func(t *testing.T) {
		p := testPanel(t, "api:\n  run: sleep 30\n")
		p.tree.Select("api")
		p.busy = true
		p.buttons()

		wants(t, p, false, false, false, false)
	})
}

func TestRun(t *testing.T) {
	p := testPanel(t, "hello:\n  run: echo hello\n")
	p.tree.Select("hello")

	p.doRun()
	// A command this window runs is its own, so the window is told how it went
	// rather than being left to find out.
	p.doing.Wait()

	c := p.byName["hello"]
	if c.how != idle {
		t.Errorf("the command is %v after ending, want it idle", c.how)
	}
	if p.shown != "hello" {
		t.Errorf("the output on the right is %q, want the command that was run", p.shown)
	}

	said := c.out.text()
	if !strings.Contains(said, "hello") {
		t.Errorf("the window heard %q, want what the command wrote", said)
	}
	if !strings.Contains(said, "ended with status 0") {
		t.Errorf("the window heard %q, want it to say how the command ended", said)
	}
}

func TestStartAndStop(t *testing.T) {
	p := testPanel(t, "api:\n  run: while true; do echo tick; sleep 0.05; done\n")
	p.tree.Select("api")

	p.doStart()
	p.doing.Wait()

	c := p.byName["api"]
	t.Cleanup(func() { runner.Stop(p.path, p.entries, "api", io.Discard) })

	if c.how != started {
		t.Fatalf("the command is %v after being started, want it started", c.how)
	}
	if c.pid == 0 {
		t.Error("the window did not take down the number of the process")
	}
	if p.shown != "api" {
		t.Errorf("the output on the right is %q, want the command that was started", p.shown)
	}
	// A started command is heard through its broadcaster, not through Runbook.
	waitFor(t, func() bool { return strings.Contains(c.out.text(), "tick") })

	p.doStop()
	p.doing.Wait()

	if c.how != idle {
		t.Errorf("the command is %v after being stopped, want it idle", c.how)
	}
	if st, err := state.Read(state.File(p.path, "api")); err == nil && st.Alive() {
		t.Error("the process is still running")
	}
}

// TestRefresh is the whole of what the window does not do on its own: a
// command started from somewhere else is not there until someone asks.
func TestRefresh(t *testing.T) {
	p := testPanel(t, "api:\n  run: while true; do echo tick; sleep 0.05; done\n")
	c := p.byName["api"]

	if err := runner.Start(p.path, p.entries, "api", io.Discard); err != nil {
		t.Fatalf("runner.Start(): %v", err)
	}
	t.Cleanup(func() { runner.Stop(p.path, p.entries, "api", io.Discard) })

	if c.how != idle {
		t.Error("the window knew a command had been started before it was asked to look")
	}

	p.refresh()

	if c.how != started {
		t.Fatalf("the command is %v after a refresh, want it started", c.how)
	}
	if !c.heard.Load() {
		t.Fatal("the window is not listening to a command that is broadcasting")
	}
	waitFor(t, func() bool { return strings.Contains(c.out.text(), "tick") })

	p.tree.Select("api")
	wants(t, p, false, false, true, true)
}

// waitFor gives a condition a couple of seconds to come true, for the moments
// where a command has to get somewhere first.
func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	for range 100 {
		if done() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("waited two seconds and it never happened")
}

func TestHeader(t *testing.T) {
	t.Run("says what is running, and of how much", func(t *testing.T) {
		p := testPanel(t, "api:\n  run: true\n\nweb:\n  run: true\n\nlint:\n  run: true\n")

		if got := p.count.Text; got != "nothing running" {
			t.Errorf("the header says %q, want %q", got, "nothing running")
		}

		p.byName["api"].how = started
		p.redraw()
		if got, want := p.count.Text, "1 of 3 commands running"; got != want {
			t.Errorf("the header says %q, want %q", got, want)
		}

		p.byName["web"].how = running
		p.redraw()
		if got, want := p.count.Text, "2 of 3 commands running"; got != want {
			t.Errorf("the header says %q, want %q", got, want)
		}
	})

	t.Run("a path is written the way a person writes it", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		if got, want := shorten(filepath.Join(home, "project", "runbook.yml")), "~/project/runbook.yml"; got != want {
			t.Errorf("shorten() = %q, want %q", got, want)
		}
		if got := shorten("/etc/runbook.yml"); got != "/etc/runbook.yml" {
			t.Errorf("shorten() = %q, want a path outside home left alone", got)
		}
		// A directory that only starts the same is not inside it.
		if got, want := shorten(home+"-else/runbook.yml"), home+"-else/runbook.yml"; got != want {
			t.Errorf("shorten() = %q, want %q", got, want)
		}
	})
}
