package cli

// primer is what `runbook iamllm` prints: everything a language model working
// in a project needs to know about Runbook, in one read.
//
// It is written for something that will act on it rather than browse it, so it
// says what the commands do to a program reading their output, and it says
// what not to do. It never looks at the runbook.yml, so it is as much use to
// something about to write one as to something using one.
const primer = `# Runbook, for a language model

Runbook runs the commands a project lists in a runbook.yml. It is not a build
system and not a process supervisor: it runs what the file says, remembers what
it started, and stays out of the way.

## The file

runbook.yml sits wherever the project keeps it, and Runbook looks for it in the
current directory unless it is given -f.

    services/api:
      description: The Spring backend
      run: mvn spring-boot:run
      dir: ./api
      env:
        PORT: 8080

    lint:
      description: Report suspicious code
      run: golangci-lint run

  <name>       what the command is called. Slashes group commands into folders
               and are part of the name: the first one above is "services/api".
  run          required. The shell command, handed to sh -c.
  description  optional. One line, which runbook list shows.
  dir          optional. Where to run it, relative to the directory the
               runbook.yml is in, which is also the default.
  env          optional. Variables on top of the ones Runbook itself was given.

## The commands

  runbook list          every command in the file, with its description
  runbook run <name>    run one here and wait for it
  runbook start <name>  start one in the background
  runbook stop <name>   end one that was started
  runbook status        which commands are running, and at which process id
  runbook logs <name>   listen to what a started command writes
  runbook -f <path>     work on a runbook.yml somewhere other than here

## What to know before using it

Use the exact name runbook list gives. Its output is made to be read by a
program when it is not going to a terminal: list prints "name<TAB>description",
and status prints "name<TAB>pid<TAB>uptime", one command per line. When nothing
is running, status prints nothing at all.

run is the one that hands you the output. It runs the command in front of you,
passes on what it writes, and exits with the status the command exited with, so
it is what to reach for when you want to see what happened. A command killed by
a signal reports 128 plus that signal, the way a shell does.

start puts the command in a session of its own, so it outlives the terminal it
was started from and a stop from anywhere still finds it. It answers with the
process id and returns. Starting a command that is already running fails rather
than starting a second one.

stop asks the command's whole process group to end, so a shell command takes
what it spawned with it, and kills it if it has not gone within five seconds.

logs hears what a started command says from now on. Nothing is written down, so
what the command said before you attached is gone for good, and there is no log
file anywhere to read instead. It ends when the command does. If you need all
of a command's output, use run.

Runbook keeps what it knows in a .runbook directory beside the runbook.yml, and
that directory ignores itself in git. Nothing is written anywhere else.

Do not run runbook with no command: that opens a window and blocks until
someone closes it.
`
