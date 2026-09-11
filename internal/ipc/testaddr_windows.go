//go:build windows

package ipc

// testWork stands in for the directory Runbook keeps a project's files in, for
// the test that only works out an address and never goes near the filesystem.
const testWork = `C:\Users\someone\.runbook\project-0123456789abcdef`

// staleContent is what a broadcaster killed outright leaves at its address: a
// port that looks like one but that nobody is listening on.
var staleContent = []byte("99999\n")
