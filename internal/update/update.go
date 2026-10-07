// Package update brings an installed Runbook up to the latest release: what
// the latest release is, fetching its .deb and making sure it is the one the
// release published, and handing it to apt.
package update

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Repo is where Runbook is published.
const Repo = "https://github.com/kaiquebahmad/runbook"

// Releases is the page of the latest release, for whoever has to fetch it by
// hand.
const Releases = Repo + "/releases/latest"

// latestAPI is where GitHub says what the latest release is.
const latestAPI = "https://api.github.com/repos/kaiquebahmad/runbook/releases/latest"

// checksums is the file every release publishes the SHA-256 of its assets in.
const checksums = "checksums.txt"

// Source is where releases are looked up and fetched from. The zero value is
// GitHub.
type Source struct {
	API    string // the latest release, as GitHub's API describes it
	Client *http.Client
}

func (s Source) api() string {
	if s.API != "" {
		return s.API
	}
	return latestAPI
}

func (s Source) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

// Release is what a release published: its tag and where each of its assets
// can be downloaded, by name.
type Release struct {
	Tag    string
	Assets map[string]string
}

// Latest asks what the latest release is.
func (s Source) Latest() (Release, error) {
	resp, err := s.client().Get(s.api())
	if err != nil {
		return Release{}, fmt.Errorf("asking for the latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("asking for the latest release: %s", resp.Status)
	}

	var body struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("reading the latest release: %w", err)
	}
	r := Release{Tag: body.Tag, Assets: map[string]string{}}
	for _, a := range body.Assets {
		r.Assets[a.Name] = a.URL
	}
	return r, nil
}

// Deb is the name of the .deb a release publishes.
func (r Release) Deb() string {
	return fmt.Sprintf("runbook_%s_amd64.deb", strings.TrimPrefix(r.Tag, "v"))
}

// Fetch downloads the asset called name into dir, and checks it against the
// SHA-256 the release published for it, so that what reaches apt is exactly
// what was released. It gives back the path of the file.
func (s Source) Fetch(r Release, name, dir string) (string, error) {
	want, err := s.checksum(r, name)
	if err != nil {
		return "", err
	}
	url, ok := r.Assets[name]
	if !ok {
		return "", fmt.Errorf("the release %s has no %s", r.Tag, name)
	}

	resp, err := s.client().Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: %s", name, resp.Status)
	}

	file := filepath.Join(dir, name)
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	sum := sha256.New()
	_, err = io.Copy(f, io.TeeReader(resp.Body, sum))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", name, err)
	}
	if got := hex.EncodeToString(sum.Sum(nil)); got != want {
		return "", fmt.Errorf("%s is not what the release published: its SHA-256 is %s, want %s", name, got, want)
	}
	return file, nil
}

// checksum is the SHA-256 the release published for the asset called name.
func (s Source) checksum(r Release, name string) (string, error) {
	url, ok := r.Assets[checksums]
	if !ok {
		return "", fmt.Errorf("the release %s has no %s to check %s against", r.Tag, checksums, name)
	}
	resp, err := s.client().Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", checksums, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: %s", checksums, resp.Status)
	}

	// sha256sum writes the sum, two spaces and the name, one file a line.
	lines := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for lines.Scan() {
		sum, file, ok := strings.Cut(lines.Text(), "  ")
		if ok && file == name {
			return sum, nil
		}
	}
	if err := lines.Err(); err != nil {
		return "", fmt.Errorf("reading %s: %w", checksums, err)
	}
	return "", fmt.Errorf("%s of %s has no sum for %s", checksums, r.Tag, name)
}

// Newer reports whether the release tagged latest comes after current. Both
// are vX.Y.Z; one that is not is never newer, and nothing is newer than it.
func Newer(latest, current string) bool {
	l, ok := parse(latest)
	if !ok {
		return false
	}
	c, ok := parse(current)
	if !ok {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// parse takes vX.Y.Z apart into its three numbers.
func parse(v string) ([3]int, bool) {
	var n [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if !strings.HasPrefix(v, "v") || len(parts) != 3 {
		return n, false
	}
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return n, false
		}
		n[i] = x
	}
	return n, true
}

// Install hands the .deb to apt, through sudo, in this terminal: sudo asks
// for the password itself, and apt for anything it wants to know.
func Install(deb string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command("sudo", "apt", "install", deb)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing %s: %w", filepath.Base(deb), err)
	}
	return nil
}
