package gui

import (
	"slices"
	"testing"

	"runbook/internal/runbookfile"
)

func TestFolders(t *testing.T) {
	f := newFolders([]runbookfile.Entry{
		{Name: "services/api"},
		{Name: "services/web"},
		{Name: "db/seed"},
		{Name: "lint"},
	})

	t.Run("the folders of a name hang off the root", func(t *testing.T) {
		want := []string{"services", "db", "lint"}
		if got := f.childrenOf(""); !slices.Equal(got, want) {
			t.Errorf("childrenOf(root) = %v, want %v", got, want)
		}
	})

	t.Run("a folder holds the commands under it", func(t *testing.T) {
		want := []string{"services/api", "services/web"}
		if got := f.childrenOf("services"); !slices.Equal(got, want) {
			t.Errorf("childrenOf(services) = %v, want %v", got, want)
		}
	})

	t.Run("the root of it all is a folder", func(t *testing.T) {
		// The tree widget walks no further than a root it is told is a leaf,
		// which is the difference between the list holding every command and
		// the list holding nothing at all.
		if !f.isBranch("") {
			t.Error("the root is not a branch, so nothing under it is ever shown")
		}
	})

	t.Run("a folder is a branch and a command is not", func(t *testing.T) {
		if !f.isBranch("services") {
			t.Error("services is not a branch")
		}
		if f.isBranch("services/api") {
			t.Error("services/api is a branch")
		}
		if f.isBranch("lint") {
			t.Error("lint is a branch")
		}
	})

	t.Run("a node is called by its last part", func(t *testing.T) {
		if got := label("services/api"); got != "api" {
			t.Errorf("label() = %q, want %q", got, "api")
		}
		if got := label("lint"); got != "lint" {
			t.Errorf("label() = %q, want %q", got, "lint")
		}
	})
}
