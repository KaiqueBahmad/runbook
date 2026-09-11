//go:build !windows

package ipc

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
)

// sockExt is what one command's address is called: a unix domain socket, which
// is a file in the filesystem and goes when the listener closes.
const sockExt = ".sock"

// maxAddr is about as long as the kernel takes a socket address, which is a
// path here. Bound rather than reported, it comes back as "invalid argument"
// from the bind, which says nothing about which path was too long.
const maxAddr = 100

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
