package main

import (
	"path/filepath"
	"testing"
)

func TestParseArgs(t *testing.T) {
	abs := func(t *testing.T, p string) string {
		t.Helper()
		full, err := filepath.Abs(p)
		if err != nil {
			t.Fatalf("filepath.Abs(%q): %v", p, err)
		}
		return full
	}

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{"no args defaults to Runbookfile", nil, abs(t, defaultRunbookfile), false},
		{"relative path is resolved", []string{"path/to/other"}, abs(t, "path/to/other"), false},
		{"absolute path is kept", []string{"/etc/Runbookfile"}, "/etc/Runbookfile", false},
		{"empty path", []string{""}, "", true},
		{"too many args", []string{"a", "b"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs(%q) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseArgs(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
