//go:build !windows

package ipc

// testWork stands in for the directory Runbook keeps a project's files in, for
// the test that only works out an address and never goes near the filesystem.
const testWork = "/home/someone/.runbook/project-0123456789abcdef"

// staleContent is what a broadcaster killed outright leaves at its address:
// nothing, since the address is the socket itself.
var staleContent []byte
