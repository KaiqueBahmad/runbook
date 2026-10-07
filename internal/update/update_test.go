package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// fakeRelease serves a release tagged tag, with a .deb holding deb and a
// checksums.txt holding sums, the way GitHub would, and gives back the source
// that finds it.
func fakeRelease(t *testing.T, tag, deb, sums string) Source {
	t.Helper()
	name := Release{Tag: tag}.Deb()

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q, "assets": [
			{"name": %q, "browser_download_url": "%s/deb"},
			{"name": "checksums.txt", "browser_download_url": "%s/sums"}
		]}`, tag, name, srv.URL, srv.URL)
	})
	mux.HandleFunc("/deb", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, deb) })
	mux.HandleFunc("/sums", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, sums) })
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return Source{API: srv.URL + "/latest", Client: srv.Client()}
}

// sha is the SHA-256 of s, the way sha256sum writes it.
func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestLatest(t *testing.T) {
	src := fakeRelease(t, "v0.5.0", "", "")
	r, err := src.Latest()
	if err != nil {
		t.Fatalf("Latest(): %v", err)
	}
	if r.Tag != "v0.5.0" {
		t.Errorf("Latest() tag = %q, want v0.5.0", r.Tag)
	}
	if r.Deb() != "runbook_0.5.0_amd64.deb" {
		t.Errorf("Deb() = %q, want runbook_0.5.0_amd64.deb", r.Deb())
	}
	if _, ok := r.Assets[r.Deb()]; !ok {
		t.Errorf("Latest() assets = %v, want the .deb among them", r.Assets)
	}
}

func TestFetch(t *testing.T) {
	const deb = "the package"
	name := Release{Tag: "v0.5.0"}.Deb()

	t.Run("the published file is kept", func(t *testing.T) {
		src := fakeRelease(t, "v0.5.0", deb, sha("other")+"  other.zip\n"+sha(deb)+"  "+name+"\n")
		r, err := src.Latest()
		if err != nil {
			t.Fatalf("Latest(): %v", err)
		}
		file, err := src.Fetch(r, name, t.TempDir())
		if err != nil {
			t.Fatalf("Fetch(): %v", err)
		}
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		if string(got) != deb {
			t.Errorf("Fetch() wrote %q, want %q", got, deb)
		}
	})

	t.Run("a file that is not the published one is refused", func(t *testing.T) {
		src := fakeRelease(t, "v0.5.0", deb, sha("something else")+"  "+name+"\n")
		r, _ := src.Latest()
		_, err := src.Fetch(r, name, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "not what the release published") {
			t.Errorf("Fetch() error = %v, want it to say the file is not the published one", err)
		}
	})

	t.Run("a file with no sum is refused", func(t *testing.T) {
		src := fakeRelease(t, "v0.5.0", deb, sha("other")+"  other.zip\n")
		r, _ := src.Latest()
		if _, err := src.Fetch(r, name, t.TempDir()); err == nil {
			t.Error("Fetch() of a file checksums.txt has no sum for succeeded")
		}
	})
}

func TestNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"v0.5.0", "v0.4.2", true},
		{"v0.10.0", "v0.9.9", true},
		{"v1.0.0", "v0.99.99", true},
		{"v0.4.2", "v0.4.2", false},
		{"v0.4.1", "v0.4.2", false},
		{"v0.5.0", "v0.4.3-0.20260921120000-9f1efea1a2b3", false},
		{"v0.5.0", "(devel)", false},
		{"0.5.0", "v0.4.2", false},
	}
	for _, tt := range tests {
		if got := Newer(tt.latest, tt.current); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}
