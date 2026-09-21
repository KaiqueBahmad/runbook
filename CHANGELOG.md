# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.1.1] - Work In Progress

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
