# Releasing

This guide explains how a version of Runbook is cut.

## Version Model

Runbook follows [Semantic Versioning](https://semver.org): **vX.Y.Z**.

- **X (major)**: a change that breaks a runbook.yml, a command, or a flag that worked before
- **Y (minor)**: something new, which leaves everything that worked still working
- **Z (patch)**: a fix, and nothing new

> **Note:** per SemVer, `0.Y.Z` is unstable — a `Y` bump (e.g. 0.1 → 0.2) may break compatibility. The promise above holds from 1.0.0 on.

The version lives in one place: the git tag. There is no version to bump in a
file. `go build` reads the tag of the commit it builds from and stamps it into
the binary, which is what `runbook --version` prints:

| Built from | `runbook --version` |
|---|---|
| the commit tagged `v0.2.0` | `runbook v0.2.0` |
| a commit after it | `runbook v0.2.1-0.20260921120000-abcdef123456` |
| either, with uncommitted changes | the same, with `+dirty` behind it |

The tag needs all three numbers — `v0.2.0`, never `v0.2`. Go does not take a
tag that is not a full semantic version, and falls back to a pseudo-version
instead.

## Cutting a Release

**Example**: releasing 0.2.0

```bash
# 1. Rename the "Work In Progress" section of CHANGELOG.md to the version and
#    today's date:
#    ## [0.2.0] - 2026-10-01

# 2. Check that everything CI checks still passes
runbook run go/check

git add CHANGELOG.md
git commit -m "chore(release): 0.2.0"

# 3. Tag the commit and push the tag
git tag v0.2.0
git push origin main v0.2.0

# 4. Check that the build says so
go build -o bin/runbook ./cmd/runbook && ./bin/runbook --version
```

Once it's out, open a new `## [0.2.1] - Work In Progress` section (or whichever
version comes next) at the top of CHANGELOG.md for what follows.

---

## Checklist Before Each Release

- [ ] CHANGELOG.md has a section for the version, dated
- [ ] `runbook run go/check` passes
- [ ] The release commit is on main, and the tree is clean
- [ ] The tag is `vX.Y.Z` and points at that commit
- [ ] `./bin/runbook --version` prints the tag, with no `+dirty`
