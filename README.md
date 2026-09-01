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

Either way, this opens the control panel: the commands on the left, grouped into folders by
the slashes in their names, and the output of one of them on the right.

Pick a command and the sidebar offers what can actually be done to it:

- **Run** — run it here. It belongs to the window and ends with it.
- **Start** — start it in the background, where it outlives the window and a `runbook stop`
  from any terminal can reach it.
- **Stop** — end it, whichever way it was set going.
- **Logs** — put what it has said on the right.

Run, Start and Logs each open that command's output on the right. A button you can't use is
greyed out: a command that isn't running can't be stopped, one that is can't be started over,
and one that has said nothing has no output to show.

Nothing starts automatically — you decide what runs and when, whether that's leaving a server up all day or firing off a migration script once.

The window doesn't go looking on its own either. On opening it connects to every command
that's broadcasting and keeps the last 1000 lines of each, and the output on the right keeps
up with what comes in — but *what is running*, and the buttons that leaves you, is what was
last asked for. **Refresh**, at the top, asks again: that's what picks up a command someone
started in another terminal, or one that has ended since.

## What Runbook is not

- Not a container runtime or sandbox — everything runs directly on your machine with your normal environment and permissions.
- Not opinionated about language or framework — if it's a shell command, Runbook can run it, server or script alike.
- Not a process supervisor for production — no restart policies, no daemonizing, no scheduling. It's a development-time convenience tool.

## Layout

The source is one small binary over a handful of packages, each with a single job:

```
cmd/runbook/           the entry point, which only hands the arguments over
internal/cli/          the command line: arguments, help, completion, and carrying the answer out
internal/gui/          the panel: the commands on the left, what one of them is saying on the right
internal/runner/       running a command here, starting one in the background, stopping it, status, logs
internal/runbookfile/  the file format, read into the entries the rest of the program works with
internal/state/        what Runbook remembers of a started command, so a later stop finds the process
internal/ipc/          the address a started command's output is broadcast at, and the broadcaster
internal/workdir/      the .runbook directory beside the Runbookfile, and tidying what is left in it
```

Everything but the entry point sits under `internal/`, so none of it can be imported from
outside the module. That's deliberate: the packages are how the program is put together,
not an API anyone is meant to build on.

To build it:

```
go build -o bin/runbook ./cmd/runbook
```

Or run `./run.sh`, which builds into `bin/` and then starts what it built, passing your
arguments along. `go test ./...` runs the tests.
