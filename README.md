# Runbook

Runbook turns a plain text file describing your project's commands into a GUI control panel — one place to run everything your project needs, instead of juggling a pile of terminal tabs and a README full of instructions.

It doesn't isolate or containerize anything. It's just an organizer: point it at commands you'd otherwise run by hand, and get buttons and live logs instead. That covers both long-running processes (servers, watchers) and one-off scripts (migrations, seeders, build tasks, lint/test runs) — anything you'd normally type into a terminal.

## Why

Most projects end up with a handful of commands scattered across terminals and docs — some you leave running, some you run once and forget:

```
cd services/api && mvn spring-boot:run
cd apps/dashboard && npm start
cd services/api && mvn flyway:migrate
./scripts/seed-db.sh
```

Runbook replaces that ritual with a single file and a single window.

## Usage

```
runbook
```

With no arguments, Runbook looks for a `Runbookfile` in the current directory. To point it at a different file, pass its path after `-f` (or `--file`):

```
runbook -f path/to/other-runbookfile
```

The path only counts when it comes after the flag — a bare `runbook path/to/other-runbookfile` exits with a usage error, and so does `-f` with nothing after it.

Relative paths are resolved against the current directory, so Runbook always works
with the full path to the file.

To print the usage text and exit without opening a window, pass `--help` (or `-h`):

```
runbook --help
```

On startup Runbook also creates a `.runbook` directory next to the Runbookfile
if it isn't there already — so it lands in whatever folder the file lives in.
That's where Runbook keeps its own files, one set per project.

The file has to exist before the window opens. If it's missing, or the path points at
a directory, Runbook says so and exits rather than starting up empty.

Either way, this opens the control panel. Each entry gets:

- A **Start** / **Stop** button (Stop is a no-op once a one-off script has already finished)
- A live-updating **log view** (stdout and stderr)
- A **status indicator** — running, stopped or crashed

Nothing starts automatically — you decide what runs and when, whether that's leaving a server up all day or firing off a migration script once.

## What Runbook is not

- Not a container runtime or sandbox — everything runs directly on your machine with your normal environment and permissions.
- Not opinionated about language or framework — if it's a shell command, Runbook can run it, server or script alike.
- Not a process supervisor for production — no restart policies, no daemonizing, no scheduling. It's a development-time convenience tool.
