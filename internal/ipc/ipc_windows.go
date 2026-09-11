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

	"runbook/internal/workdir"
)

// On Windows, Runbook reaches a broadcaster over a TCP connection on
// localhost. Each broadcaster picks a free port and records it in a
// file, so that a listener can find it. This avoids the complexity of
// named pipes while keeping the same address-per-command model.

const (
	sockDirName = "sock"
	sockExt     = ".port"
)

// maxAddr is not relevant on Windows (TCP addresses are short), but
// we keep the limit for consistency.
const maxAddr = 100

// dir is where the addresses of one runbook.yml's commands live.
func dir(work string) string {
	return filepath.Join(work, sockDirName)
}

// Addr is the file that holds the port the broadcaster of one command
// is listening on. A command name is a path already, so its folders
// become directories.
func Addr(work, name string) string {
	return filepath.Join(dir(work), filepath.FromSlash(name)+sockExt)
}

// Listen takes up the address a broadcaster is reached at. It picks a
// free port, writes it to the file at addr, and returns the listener
// on that port. The port file is removed when the listener is closed.
func Listen(addr string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(addr), 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", filepath.Dir(addr), err)
	}
	// Remove a leftover address file. It may be left by a broadcaster
	// that was killed outright.
	if err := os.Remove(addr); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	// Pick a free port by listening on :0.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listening on a free port: %w", err)
	}

	// Write the port to the file so that Dial can find it.
	addrPort := l.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(addr, []byte(strconv.Itoa(addrPort)+"\n"), 0o600); err != nil {
		l.Close()
		return nil, fmt.Errorf("writing address for %s: %w", addr, err)
	}

	return &cleanListener{l, addr}, nil
}

// cleanListener wraps a net.Listener and removes the port file when closed.
type cleanListener struct {
	net.Listener
	addr string
}

func (cl *cleanListener) Close() error {
	err := cl.Listener.Close()
	os.Remove(cl.addr) // best-effort cleanup
	return err
}

// Dial connects to the broadcaster of a command by reading the port
// from its address file.
func Dial(addr string) (net.Conn, error) {
	data, err := os.ReadFile(addr)
	if err != nil {
		return nil, err
	}
	portStr := strings.TrimSpace(string(data))
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("bad port in %s: %q", addr, portStr)
	}
	return net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
}

// Sweep forgets the addresses of one runbook.yml's commands that
// nobody is listening at any more: the port files whose broadcaster
// is gone, and the folders left empty once those are gone.
func Sweep(work string) error {
	return workdir.SweepUnder(dir(work), dead)
}

// dead reports whether an address file is one whose broadcaster is
// gone. It tries to connect: if nobody is there, the address is stale.
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
