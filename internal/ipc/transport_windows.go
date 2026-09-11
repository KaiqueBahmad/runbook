//go:build windows

package ipc

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// sockExt is what one command's address is called. Windows has no socket in
// the filesystem to bind, so the broadcaster listens on a port of the loopback
// that the machine hands out, and the address holds that port.
const sockExt = ".port"

// Listen takes up the address a broadcaster is reached at: a port nothing else
// is on, written where Dial will look for it.
func Listen(addr string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(addr), 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", filepath.Dir(addr), err)
	}
	// An address left behind by a broadcaster that was killed outright is in
	// the way, and there is nothing behind it to protect: the one command it
	// belongs to is not running, or start would not have got here.
	if err := os.Remove(addr); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	// Port 0 is the machine being asked for one that is free.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listening on a free port: %w", err)
	}

	port := l.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(addr, []byte(strconv.Itoa(port)+"\n"), 0o600); err != nil {
		l.Close()
		return nil, fmt.Errorf("writing %s: %w", addr, err)
	}
	return &tidyListener{l, addr}, nil
}

// tidyListener takes the address back out of the filesystem when the listener
// closes, so that a broadcaster that ends leaves nothing behind — which is
// what a unix socket does on its own.
type tidyListener struct {
	net.Listener
	addr string
}

func (l *tidyListener) Close() error {
	err := l.Listener.Close()
	os.Remove(l.addr) // the address is no use without the listener
	return err
}

// Dial connects to the broadcaster of a command. It fails when there is
// nobody at the address, which is what a command that is not running looks
// like: no port written, or nothing answering on the port that is.
func Dial(addr string) (net.Conn, error) {
	data, err := os.ReadFile(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("the address %s holds %q, which is no port", addr, strings.TrimSpace(string(data)))
	}
	return net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
}
