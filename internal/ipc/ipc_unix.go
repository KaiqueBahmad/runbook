//go:build !windows

// Package ipc is how Runbook reaches the broadcaster of a started command: the
// address it listens at, and the broadcaster itself.
package ipc

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"

	"runbook/internal/workdir"
)

// Runbook reaches a broadcaster over an address only this machine can see: a
// unix domain socket here, a named pipe on Windows. Neither takes a port, so a
// started command adds nothing to what the machine is listening to, and a
// broadcaster is never mistaken for something that has been left open.
//
// TODO: write the Windows half of this, an ipc_windows.go with named pipes.

// sockDirName names the directory the addresses of one runbook.yml's commands
// live in, and sockExt the address of one command inside it.
const (
	sockDirName = "sock"
	sockExt     = ".sock"
)

// maxAddr is about as long as the kernel takes a socket address, which is a
// path here. Bound rather than reported, it comes back as "invalid argument"
// from the bind, which says nothing about which path was too long.
const maxAddr = 100

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

// Listen takes up the address a broadcaster is reached at.
func Listen(addr string) (net.Listener, error) {
	if len(addr) > maxAddr {
		return nil, fmt.Errorf("the address %s is %d characters, and a socket takes %d", addr, len(addr), maxAddr)
	}
	if err := os.MkdirAll(filepath.Dir(addr), 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", filepath.Dir(addr), err)
	}
	// An address left behind by a broadcaster that was killed outright is in
	// the way of the bind, and there is nothing behind it to protect: the one
	// command it belongs to is not running, or start would not have got here.
	if err := os.Remove(addr); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	l, err := net.Listen("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", addr, err)
	}
	// Go takes the address back out of the filesystem when the listener is
	// closed, so a broadcaster that ends leaves nothing behind.
	return l, nil
}

// Dial connects to the broadcaster of a command. It fails when there is
// nobody at the address, which is what a command that is not running looks
// like.
func Dial(addr string) (net.Conn, error) {
	return net.Dial("unix", addr)
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
