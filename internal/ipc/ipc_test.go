package ipc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddr(t *testing.T) {
	want := filepath.Join(testWork, sockDirName, "services", "api"+sockExt)
	if got := Addr(testWork, "services/api"); got != want {
		t.Errorf("Addr(%q, %q) = %q, want %q", testWork, "services/api", got, want)
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
	if err := os.WriteFile(stale, staleContent, 0o600); err != nil {
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
