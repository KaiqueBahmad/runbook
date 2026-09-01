package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseRunbookfile(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Entry
	}{
		{
			name: "empty file",
			in:   "",
			want: nil,
		},
		{
			name: "run on its own leaves the optional fields unset",
			in: `api:
  run: npm start
`,
			want: []Entry{{Name: "api", Run: "npm start", Description: "", Dir: "", Env: nil}},
		},
		{
			name: "several commands",
			in: `services/api:
  run: mvn spring-boot:run
lint:
  run: golangci-lint run
`,
			want: []Entry{
				{Name: "services/api", Run: "mvn spring-boot:run"},
				{Name: "lint", Run: "golangci-lint run"},
			},
		},
		{
			name: "description",
			in: `api:
  description: The backend, on port 8080
  run: npm start
`,
			want: []Entry{{
				Name:        "api",
				Run:         "npm start",
				Description: "The backend, on port 8080",
			}},
		},
		{
			name: "every field",
			in: `api:
  description: The Spring backend
  dir: services/api
  run: mvn spring-boot:run
  env:
    PORT: 8080
    LOG_LEVEL: debug
`,
			want: []Entry{{
				Name:        "api",
				Run:         "mvn spring-boot:run",
				Description: "The Spring backend",
				Dir:         "services/api",
				Env:         map[string]string{"PORT": "8080", "LOG_LEVEL": "debug"},
			}},
		},
		{
			name: "file order is kept",
			in: `db/migrate:
  run: mvn flyway:migrate
services/api:
  dir: services/api
  run: mvn spring-boot:run
db/seed:
  run: ./scripts/seed-db.sh
`,
			want: []Entry{
				{Name: "db/migrate", Run: "mvn flyway:migrate"},
				{Name: "services/api", Run: "mvn spring-boot:run", Dir: "services/api"},
				{Name: "db/seed", Run: "./scripts/seed-db.sh"},
			},
		},
		{
			name: "a field after the env block",
			in: `api:
  env:
    PORT: 8080
  run: npm start
`,
			want: []Entry{{
				Name: "api",
				Run:  "npm start",
				Env:  map[string]string{"PORT": "8080"},
			}},
		},
		{
			name: "blank lines and comments",
			in: `# the API

api:
   # indented comment
  run: npm start

  # another one
web:
  run: npm run dev
`,
			want: []Entry{
				{Name: "api", Run: "npm start"},
				{Name: "web", Run: "npm run dev"},
			},
		},
		{
			name: "a hash inside a command is kept",
			in:   "api:\n  run: echo '# not a comment'\n",
			want: []Entry{{Name: "api", Run: "echo '# not a comment'"}},
		},
		{
			name: "spaces inside a folder are kept",
			in:   "my services/the api:\n  run: npm start\n",
			want: []Entry{{Name: "my services/the api", Run: "npm start"}},
		},
		{
			name: "empty env block",
			in: `api:
  run: npm start
  env:
`,
			want: []Entry{{Name: "api", Run: "npm start", Env: map[string]string{}}},
		},
		{
			name: "empty environment value",
			in: `api:
  run: npm start
  env:
    LOG_LEVEL:
`,
			want: []Entry{{
				Name: "api",
				Run:  "npm start",
				Env:  map[string]string{"LOG_LEVEL": ""},
			}},
		},
		{
			name: "missing trailing newline",
			in:   "api:\n  run: npm start",
			want: []Entry{{Name: "api", Run: "npm start"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRunbookfile(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("parseRunbookfile(): %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRunbookfile() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseRunbookfileErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string // a fragment the error has to mention
	}{
		{"no colon", "api npm start\n", "line 1"},
		{"empty name", ":\n  run: npm start\n", "name is empty"},
		{"empty folder", "services//api:\n  run: npm start\n", "empty folder"},
		{"blank folder", "services/ /api:\n  run: npm start\n", "empty folder"},
		{"space before a folder", "services/ api:\n  run: npm start\n", "spaces around"},
		{"space after a folder", "services /api:\n  run: npm start\n", "spaces around"},
		{"space before the colon", "api :\n  run: npm start\n", "spaces around"},
		{"duplicate command", "api:\n  run: a\napi:\n  run: b\n", "line 1"},
		{"command that is also a folder", "db:\n  run: psql\ndb/seed:\n  run: ./seed.sh\n", "folder"},
		{"command on the name line", "api: npm start\n", "takes no command here"},
		{"block without a run", "api:\n  dir: services/api\n", "no run command"},
		{"unknown field", "api:\n  runn: npm start\n", `unknown field "runn"`},
		{"run set twice", "api:\n  run: npm start\n  run: npm run dev\n", "set twice"},
		{"dir set twice", "api:\n  run: npm start\n  dir: a\n  dir: b\n", "set twice"},
		{"description set twice", "api:\n  run: x\n  description: a\n  description: b\n", "set twice"},
		{"empty description", "api:\n  run: x\n  description:\n", "description is empty"},
		{"env set twice", "api:\n  run: npm start\n  env:\n  env:\n", "set twice"},
		{"empty run", "api:\n  run:\n", "run is empty"},
		{"empty dir", "api:\n  run: npm start\n  dir:\n", "dir is empty"},
		{"env with a value", "api:\n  run: npm start\n  env: PORT=8080\n", "takes no value"},
		{"duplicate variable", "api:\n  run: x\n  env:\n    PORT: 1\n    PORT: 2\n", "set twice"},
		{"indented line without a command", "  run: npm start\n", "does not belong"},
		{"inconsistent field indent", "api:\n  run: npm start\n    dir: services/api\n", "indented"},
		{"inconsistent variable indent", "api:\n  run: x\n  env:\n    PORT: 1\n      LOG: 2\n", "indented"},
		{"tab indent", "api:\n\trun: npm start\n", "tabs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRunbookfile(strings.NewReader(tt.in))
			if err == nil {
				t.Fatalf("parseRunbookfile() = %+v, want an error", got)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("parseRunbookfile() error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestReadRunbookfile(t *testing.T) {
	dir := t.TempDir()
	write := func(t *testing.T, name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
		return path
	}

	t.Run("reads the commands", func(t *testing.T) {
		path := write(t, "Runbookfile", "api:\n  run: npm start\n")

		got, err := readRunbookfile(path)
		if err != nil {
			t.Fatalf("readRunbookfile(): %v", err)
		}
		want := []Entry{{Name: "api", Run: "npm start"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("readRunbookfile() = %+v, want %+v", got, want)
		}
	})

	t.Run("parse errors name the file", func(t *testing.T) {
		path := write(t, "Broken", "api:\n  dir: services/api\n")

		_, err := readRunbookfile(path)
		if err == nil {
			t.Fatal("readRunbookfile() error = nil, want an error")
		}
		if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "line 1") {
			t.Errorf("readRunbookfile() error = %v, want it to mention %s and the line", err, path)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if _, err := readRunbookfile(filepath.Join(dir, "nope")); err == nil {
			t.Fatal("readRunbookfile() error = nil, want an error")
		}
	})
}

func TestPrintEntries(t *testing.T) {
	entries := []Entry{
		{
			Name:        "services/api",
			Run:         "mvn spring-boot:run",
			Description: "The Spring backend",
			Dir:         "services/api",
			Env:         map[string]string{"PORT": "8080", "LOG_LEVEL": "debug"},
		},
		{Name: "lint", Run: "golangci-lint run"},
	}

	// The variables are sorted by name, since a map has no order to keep.
	want := `services/api:
  description: The Spring backend
  run: mvn spring-boot:run
  dir: services/api
  env:
    LOG_LEVEL: debug
    PORT: 8080
lint:
  run: golangci-lint run
`

	var out bytes.Buffer
	printEntries(&out, entries)
	if out.String() != want {
		t.Errorf("printEntries() =\n%s\nwant\n%s", out.String(), want)
	}
}

// The example file is the documentation for the syntax, so it has to stay
// something Runbook can actually read.
func TestRunbookfileExample(t *testing.T) {
	entries, err := readRunbookfile("Runbookfile")
	if err != nil {
		t.Fatalf("readRunbookfile(): %v", err)
	}
	if len(entries) == 0 {
		t.Error("Runbookfile has no commands")
	}
}

func TestPrintNames(t *testing.T) {
	entries := []Entry{
		{Name: "services/api", Run: "mvn spring-boot:run", Description: "The Spring backend"},
		{Name: "db/seed", Run: "./seed.sh", Description: "Fill the database"},
		{Name: "lint", Run: "golangci-lint run"},
	}

	tests := []struct {
		name  string
		align bool
		want  string
	}{
		{
			// The column is as wide as the longest name that has something to
			// line up against, and a command with no description is left alone.
			name:  "aligned",
			align: true,
			want: "services/api  The Spring backend\n" +
				"db/seed       Fill the database\n" +
				"lint\n",
		},
		{
			name:  "not aligned",
			align: false,
			want: "services/api\tThe Spring backend\n" +
				"db/seed\tFill the database\n" +
				"lint\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			printNames(&out, entries, tt.align)
			if out.String() != tt.want {
				t.Errorf("printNames(align=%v) =\n%q\nwant\n%q", tt.align, out.String(), tt.want)
			}
		})
	}
}

func TestPrintNamesWidth(t *testing.T) {
	// The column is measured in characters, not bytes, so an accent does not
	// push the description of the line below it out of place.
	entries := []Entry{
		{Name: "café", Description: "a"},
		{Name: "abcd", Description: "b"},
	}

	var out bytes.Buffer
	printNames(&out, entries, true)
	want := "café  a\nabcd  b\n"
	if out.String() != want {
		t.Errorf("printNames() = %q, want %q", out.String(), want)
	}
}
