package runner

import (
	"bytes"
	"path/filepath"
	"testing"

	"runbook/internal/runbookfile"
)

func TestInspect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runbook.yml")
	base := filepath.Dir(path)
	entries := []runbookfile.Entry{
		{Name: "lint", Run: "golangci-lint run"},
		{Name: "api", Run: "go run main.go", Dir: "./api"},
		{Name: "db/reset", Run: "dropdb app\ncreatedb app", Dir: "."},
	}

	tests := []struct {
		name string
		want string
	}{
		{"lint", "golangci-lint run\n"},
		{"api", cdTo(filepath.Join(base, "api")) + " && go run main.go\n"},
		{"db/reset", cdTo(base) + " && dropdb app\ncreatedb app\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Inspect(path, entries, tt.name, &buf); err != nil {
				t.Fatalf("Inspect(): %v", err)
			}
			if buf.String() != tt.want {
				t.Errorf("Inspect() wrote %q, want %q", buf.String(), tt.want)
			}
		})
	}

	if err := Inspect(path, entries, "nope", &bytes.Buffer{}); err == nil {
		t.Error("Inspect() of an unknown name: error = nil, want one")
	}
}
