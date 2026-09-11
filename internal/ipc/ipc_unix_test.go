//go:build !windows

package ipc

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestListenAddrTooLong is the bound on a unix socket address, which is a path
// the kernel takes about a hundred characters of. A named address on Windows
// has no such limit.
func TestListenAddrTooLong(t *testing.T) {
	addr := filepath.Join(t.TempDir(), strings.Repeat("l", maxAddr), "api"+sockExt)

	_, err := Listen(addr)
	if err == nil {
		t.Fatal("Listen() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "characters") {
		t.Errorf("Listen() error = %v, want it to say the address is too long", err)
	}
}
