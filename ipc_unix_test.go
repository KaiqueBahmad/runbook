//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIPCAddr(t *testing.T) {
	path := "/home/kaique/project/Runbookfile"
	want := "/home/kaique/project/.runbook/Runbookfile.sock/services/api.sock"
	if got := ipcAddr(path, "services/api"); got != want {
		t.Errorf("ipcAddr(%q, %q) = %q, want %q", path, "services/api", got, want)
	}
}

func TestListenIPC(t *testing.T) {
	t.Run("takes the address up and gives it back", func(t *testing.T) {
		addr := testAddr(t.TempDir())

		l, err := listenIPC(addr)
		if err != nil {
			t.Fatalf("listenIPC(): %v", err)
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

		l, err := listenIPC(addr)
		if err != nil {
			t.Fatalf("listenIPC() over a leftover address: %v", err)
		}
		l.Close()
	})

	t.Run("says when the address is too long", func(t *testing.T) {
		addr := filepath.Join(t.TempDir(), strings.Repeat("l", maxAddr), "api.sock")

		_, err := listenIPC(addr)
		if err == nil {
			t.Fatal("listenIPC() error = nil, want an error")
		}
		if !strings.Contains(err.Error(), "characters") {
			t.Errorf("listenIPC() error = %v, want it to say the address is too long", err)
		}
	})
}

func TestSweepAddresses(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "Runbookfile")

	// One with a broadcaster behind it, one a killed broadcaster left behind.
	live := ipcAddr(path, "api")
	l, err := listenIPC(live)
	if err != nil {
		t.Fatalf("listenIPC(): %v", err)
	}
	defer l.Close()

	stale := ipcAddr(path, "old/web")
	if err := os.MkdirAll(filepath.Dir(stale), 0o700); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(stale), err)
	}
	if err := os.WriteFile(stale, nil, 0o600); err != nil {
		t.Fatalf("writing %s: %v", stale, err)
	}

	if err := sweep(path); err != nil {
		t.Fatalf("sweep(): %v", err)
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
