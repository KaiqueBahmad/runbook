package cli

import (
	"fmt"
	"runtime/debug"
)

// version is the semantic version of this build. Left empty, it is read from
// what go build stamped into the binary: the git tag of the commit it was built
// from, such as v1.2.0, or a pseudo-version behind the last tag when the commit
// has none of its own. A release can set it outright with
// -ldflags "-X runbook/internal/cli.version=v1.2.0".
var version string

// currentVersion is the version --version prints.
func currentVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

// channel is how this build reaches whoever runs it, set by the release that
// packs it with -ldflags "-X runbook/internal/cli.channel=deb": "deb" for the
// .deb, "tar" for the .tar.gz and "zip" for the Windows .zip. Left empty, it
// is a build from source. It is what decides how update brings it up to date.
var channel string

// The channels a release sets.
const (
	channelDeb = "deb"
	channelTar = "tar"
	channelZip = "zip"
)

// versionLine is what --version prints: the version, and the channel when a
// release set one.
func versionLine() string {
	if channel == "" {
		return "runbook " + currentVersion()
	}
	return fmt.Sprintf("runbook %s (%s)", currentVersion(), channel)
}
