package gui

import (
	"slices"
	"strings"

	"runbook/internal/runbookfile"
)

// folders is the list on the left of the window: a command's name is a path
// already, so everything before the last slash is a folder to put it in.
//
// A node carries the whole path down to it, so "services/api" hangs under
// "services", and the root of it all is the empty name. That is what the tree
// widget asks for, and it is what a command is called, so the two need no
// translating between them.
type folders struct {
	children map[string][]string
}

// newFolders puts every command of a runbook.yml in its folder, keeping the
// order the file lists them in.
func newFolders(entries []runbookfile.Entry) *folders {
	f := &folders{children: map[string][]string{}}
	for _, entry := range entries {
		parent := ""
		for _, part := range strings.Split(entry.Name, "/") {
			node := part
			if parent != "" {
				node = parent + "/" + part
			}
			f.hang(parent, node)
			parent = node
		}
	}
	return f
}

// hang puts node under parent, unless another command put it there already.
func (f *folders) hang(parent, node string) {
	if slices.Contains(f.children[parent], node) {
		return
	}
	f.children[parent] = append(f.children[parent], node)
}

// childrenOf and isBranch are what the tree widget asks of the list. A node
// with something under it is a folder, and so is the root of the whole thing,
// the empty name: the widget walks no further than a root it is told is a
// leaf, so getting that one wrong leaves the list empty.
func (f *folders) childrenOf(node string) []string { return f.children[node] }
func (f *folders) isBranch(node string) bool       { return len(f.children[node]) > 0 }

// label is what a node is called where it hangs, which is the last part of it.
func label(node string) string {
	if cut := strings.LastIndex(node, "/"); cut >= 0 {
		return node[cut+1:]
	}
	return node
}
