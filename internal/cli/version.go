package cli

import "runtime/debug"

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
