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

runbook.yml sits wherever the project keeps it, usually at the top. Unless it
is given -f or -u, Runbook looks for it in the current directory, then in the
directory above, and so on up to the root, and works with the first one it
finds. -f selects the path that follows it; -u selects ~/runbook.yml. So the
commands of a project answer from anywhere inside it, and paths in the file are
still read against the directory the file itself is in, not the one you happen
to be standing in.

    services/api:
      description: The Spring backend
      run: mvn spring-boot:run
      dir: ./api
      env:
        PORT: 8080

    lint:
      description: Report suspicious code
      run: golangci-lint run

    db/reset:
      description: Drop the database and build it back
      run: |
        dropdb app
        createdb app
        psql app < schema.sql

  <name>       what the command is called. Slashes group commands into folders
               and are part of the name: the first one above is "services/api".
  run          required. The shell command, handed to sh -c. One that does
               not fit on a line is written as a "|", with the lines of the
               command indented under it, as db/reset is above. They are run
               as written, comments and blank lines and all. A "|" anywhere
               else on the line is the shell's pipe, not a block.
  description  optional. One line, which runbook list shows.
  dir          optional. Where to run it, relative to the directory the
               runbook.yml is in. Without it, the command runs in the
               current directory.
  env          optional. Variables on top of the ones Runbook itself was given.

## The commands

  runbook gui           open the control panel window
  runbook list          every command in the file, with its description
  runbook run <name>    run one here and wait for it
  runbook start <name>...  start them in the background
  runbook stop <name>...   end ones that were started
  runbook status        which commands are running, and at which process id
  runbook logs [name]   listen to what a started command writes, or with no
                        name to every command that is running at once
  runbook -f <path>     work on a runbook.yml somewhere other than here
  runbook -u            work on ~/runbook.yml
  runbook --version     which Runbook this is, as a semantic version

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
process id and returns. Starting a command that is already running leaves it as
it is and says so, rather than starting a second one; that is not a failure, so
start exits 0 and a script can start what it needs without checking first.

start and stop take any number of names and see to each in the order given. A
name the runbook.yml does not have fails before anything is done; otherwise one
that fails is reported, the rest are still seen to, and it exits 1 at the end.

stop asks the command's whole process group to end, so a shell command takes
what it spawned with it, and kills it if it has not gone within five seconds.

logs hears what a started command says from now on. Nothing is written down, so
what the command said before you attached is gone for good, and there is no log
file anywhere to read instead. It ends when the command does. If you need all
of a command's output, use run.

logs with no name hears every command that is running at that moment, and puts
the name of the command in front of each line: "name<TAB>line" when it is not
going to a terminal. It ends when the last of those commands does, and a command
started after it began is not among them. It fails when nothing is running.

Runbook keeps what it knows in a .runbook directory in the home directory of
whoever is running it, one directory per runbook.yml. It writes nothing into the
project itself, so a project is left exactly as it was found.

Do not run runbook gui: that opens a window and blocks until someone closes
it. Runbook with no command at all opens nothing — it prints its help and
exits.
`
