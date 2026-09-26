# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- `runbook start` and `runbook stop` take several names, `runbook start api web db`, and see to each in turn. A name the file does not have stops it before anything starts; a command that fails is reported and the rest still go ahead, with exit status 1 at the end

### Changed
- `runbook start` on a command that is already running says so and exits 0 instead of failing, so `runbook start one && runbook start two` goes on to `two` whatever the state of `one`

## [0.2.0] - 2026-09-23

### Changed
- A command with no `dir` runs in the current directory, not the one the `runbook.yml` is in. A command that relied on the old default wants `dir: .`

### Fixed
- The Linux binary and the .deb start on systems with glibc older than 2.38, such as Ubuntu 22.04

## [0.1.1] - 2026-09-21

### Added
- A .deb package in every release, with the bash and fish completion

## [0.1.0] - 2026-09-21

The first release: everything Runbook does so far.

### Added
- `runbook.yml`, a list of named commands, each with a `run` (one line or a multiline script) and an optional `description`, `dir` and `env`
- `runbook gui`, the control panel: the commands on the left, grouped into folders by the slashes in their names, and the output of one of them on the right, with Run, Start, Stop and Logs
- Logs in the panel that can be selected and copied, and that hold still while being read
- `runbook list`, `run`, `start`, `stop`, `status` and `logs` in the terminal; `logs` with no name listens to every command that is running at once
- `runbook completion` for bash, zsh and fish, completing the commands of the runbook.yml being typed against
- `runbook iamllm`, a page written for a language model
- Looking for the runbook.yml in the current directory and every directory above it, `-f`/`--file` to pick another one and `-u`/`--user` for `~/runbook.yml`
- What Runbook remembers kept in `~/.runbook`, one directory per runbook.yml, so a project is left exactly as it was found
- `runbook --version`/`-v`, the version of the build, read from the git tag it was built from
- Windows support
- A GitHub release for every `vX.Y.Z` tag, with binaries for Linux and Windows
