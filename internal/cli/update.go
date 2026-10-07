package cli

import (
	"fmt"
	"io"
	"os"

	"runbook/internal/update"
)

// runUpdate brings this binary up to the latest release, which it can do only
// for the .deb. Any other build says where the latest is to be had, and exits
// with a failure, since nothing was updated.
func runUpdate(stdin io.Reader, stdout, stderr io.Writer) int {
	if channel != channelDeb {
		fmt.Fprint(stderr, elsewhere(channel))
		return 1
	}
	return report(updateDeb(update.Source{}, currentVersion(), stdin, stdout, stderr))
}

// elsewhere is what update says to a build it cannot update: where the
// latest one is.
func elsewhere(channel string) string {
	switch channel {
	case channelTar:
		return "Using the binary from the .tar.gz release.\n" +
			"Latest builds available at " + update.Releases + "\n"
	case channelZip:
		return "Using the binary from the Windows .zip release.\n" +
			"Latest builds available at " + update.Releases + "\n"
	case "":
		return "Using a binary built from source.\n" +
			"More about building from source at " + update.Repo + "#install\n"
	}
	return fmt.Sprintf("Using a %s build, which runbook update does not know.\n", channel) +
		"Latest builds available at " + update.Releases + "\n"
}

// updateDeb fetches the .deb of the latest release and has apt install it,
// unless what is running is the latest already.
func updateDeb(src update.Source, current string, stdin io.Reader, stdout, stderr io.Writer) error {
	latest, err := src.Latest()
	if err != nil {
		return err
	}
	if !update.Newer(latest.Tag, current) {
		fmt.Fprintf(stdout, "Using %s, the latest.\n", current)
		return nil
	}
	fmt.Fprintf(stdout, "Using %s, the latest is %s.\n", current, latest.Tag)

	// apt reads the file as a user of its own, so the directory is left open
	// to it.
	dir, err := os.MkdirTemp("", "runbook-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0o755); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Downloading %s\n", latest.Deb())
	deb, err := src.Fetch(latest, latest.Deb(), dir)
	if err != nil {
		return err
	}
	if err := update.Install(deb, stdin, stdout, stderr); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Installed %s\n", latest.Tag)
	return nil
}
