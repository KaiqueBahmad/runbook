//go:build windows

package ipc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddr(t *testing.T) {
	work := "C:\\Users\\someone\\.runbook\\project-0123456789abcdef"
	want := work + "\\sock\\services\\api.port"
	if got := Addr(work, "services/api"); got != want {
		t.Errorf("Addr(%q, %q) = %q, want %q", work, "services/api", got, want)
	}
}

func TestListen(t *testing.T) {
	t.Run("takes the address up and gives it back", func(t *testing.T) {
		addr := testAddr(t.TempDir())

		l, err := Listen(addr)
		if err != nil {
			t.Fatalf("Listen(): %v", err)
		}
		if !exists(addr) {
			t.Error("the address is not there to connect to")
		}
		if err := l.Close(); err != nil {
			t.Errorf("closing the listener: %v", err)
		}
		if exists(addr) {
			t.Error("the address was left behind by a broadcaster that ended")
		}
	})

	t.Run("binds over one left behind", func(t *testing.T) {
		addr := testAddr(t.TempDir())
		if err := os.MkdirAll(filepath.Dir(addr), 0o700); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(addr), err)
		}
		// What a broadcaster killed outright leaves in the way.
		if err := os.WriteFile(addr, nil, 0o600); err != nil {
			t.Fatalf("writing %s: %v", addr, err)
		}

		l, err := Listen(addr)
		if err != nil {
			t.Fatalf("Listen() over a leftover address: %v", err)
		}
		l.Close()
	})
}

func TestSweep(t *testing.T) {
	work := t.TempDir()

	// One with a broadcaster behind it, one a killed broadcaster left behind.
	live := Addr(work, "api")
	l, err := Listen(live)
	if err != nil {
		t.Fatalf("Listen(): %v", err)
	}
	defer l.Close()

	stale := Addr(work, "old/web")
	if err := os.MkdirAll(filepath.Dir(stale), 0o700); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(stale), err)
	}
	if err := os.WriteFile(stale, []byte("99999\n"), 0o600); err != nil {
		t.Fatalf("writing %s: %v", stale, err)
	}

	if err := Sweep(work); err != nil {
		t.Fatalf("Sweep(): %v", err)
	}
	if exists(stale) {
		t.Error("an address with nobody behind it was kept")
	}
	if exists(filepath.Dir(stale)) {
		t.Error("the folder left empty by it was kept")
	}
	if !exists(live) {
		t.Error("the address of a running command was swept away")
	}
}

func TestAddrTooLong(t *testing.T) {
	// On Windows, TCP addresses are not limited by path length like Unix sockets,
	// but we still test the addr validation logic.
	addr := filepath.Join(t.TempDir(), strings.Repeat("l", 200), "api.port")
	if err := os.MkdirAll(filepath.Dir(addr), 0o700); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(addr), err)
	}

	// This should still work since Windows doesn't have the same addr length limit.
	l, err := Listen(addr)
	if err != nil {
		t.Fatalf("Listen() with long addr: %v", err)
	}
	l.Close()
}
