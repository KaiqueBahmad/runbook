// Package ipc is how Runbook reaches the broadcaster of a started command: the
// address it listens at, and the broadcaster itself.
//
// Runbook reaches a broadcaster over an address only this machine can see: a
// unix domain socket, or a port on the loopback that nothing outside can
// answer. A started command adds nothing to what the machine is listening to,
// and a broadcaster is never mistaken for something that has been left open.
// What differs between the two lives in transport_unix.go and its Windows
// twin; everything here is the same on both.
package ipc

import (
	"path/filepath"
	"strings"

	"runbook/internal/workdir"
)

// sockDirName names the directory the addresses of one runbook.yml's commands
// live in. What one address inside it is called is sockExt, which the two
// transports name for themselves.
const sockDirName = "sock"

// dir is where the addresses of one runbook.yml's commands live, beside the
// state files, inside the directory Runbook keeps for that file.
func dir(work string) string {
	return filepath.Join(work, sockDirName)
}

// Addr is the address the broadcaster of one command listens on. A command
// name is a path already, so its folders become directories.
func Addr(work, name string) string {
	return filepath.Join(dir(work), filepath.FromSlash(name)+sockExt)
}

// Sweep forgets the addresses of one runbook.yml's commands that nobody is
// behind any more: what a broadcaster killed outright left in the filesystem,
// and the folders left empty once those are gone.
func Sweep(work string) error {
	return workdir.SweepUnder(dir(work), dead)
}

// dead reports whether an address is one nobody is listening at. Connecting
// is the whole of the test: an address is only there while a broadcaster holds
// it, and one that refuses the connection is a leftover.
func dead(file string) bool {
	if !strings.HasSuffix(file, sockExt) {
		return false
	}
	conn, err := Dial(file)
	if err != nil {
		return true
	}
	conn.Close()
	return false
}
