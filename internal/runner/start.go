package runner

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"runbook/internal/ipc"
	"runbook/internal/runbookfile"
	"runbook/internal/state"
	"runbook/internal/workdir"
)

// BroadcastCommand is what Runbook calls itself with to carry the output of a
// started command to whoever is listening for it. It is not for people to
// type, so it is left out of the help and the completion scripts, and it takes
// an address rather than the name of a command.
const BroadcastCommand = "broadcast"

// grace is how long a stopped command has to end on its own after SIGTERM,
// before Runbook stops asking and kills it.
const grace = 5 * time.Second

// bind is how long the broadcaster has to take up its address before start
// gives up on it.
const bind = 2 * time.Second

// startEntry starts a command in the background and records its process id, so
// that a stop from another terminal can find it again.
//
// It starts two processes, each in a session of its own: the command, and the
// broadcaster that carries its output to anyone who asks for it. A session
// gives the command a process group to lead, which is what a shell command's
// own children join, so stopping it reaches the whole tree; and it takes both
// of them off the terminal Runbook was typed at, so a Ctrl-C there ends
// neither.
//
// The two are joined by a pipe. Runbook holds the write end until the command
// has it too, and then lets go, so that the broadcaster sees the output end
// exactly when the command does and not before.
func startEntry(entry runbookfile.Entry, base, stateFile, addr string) (int, error) {
	if st, err := state.Read(stateFile); err == nil && st.Alive() {
		return 0, fmt.Errorf("%s is already running (pid %d)", entry.Name, st.PID)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}

	r, w, err := os.Pipe()
	if err != nil {
		return 0, fmt.Errorf("opening a pipe for %s: %w", entry.Name, err)
	}
	caster, trouble, err := startBroadcaster(addr, r)
	r.Close()
	if err != nil {
		w.Close()
		return 0, err
	}
	defer trouble.Close()
	// From here on the broadcaster ends of its own accord when this is closed,
	// so every way out below takes it along.
	defer w.Close()

	// The address is waited for rather than assumed: someone who types logs
	// straight after start should find the broadcaster already there.
	if err := waitAddr(addr, bind); err != nil {
		caster.Process.Kill()
		caster.Wait()
		if said := whyNot(trouble); said != "" {
			return 0, fmt.Errorf("%s: %s", entry.Name, said)
		}
		return 0, fmt.Errorf("%s: %w", entry.Name, err)
	}

	cmd := exec.Command(shell, "-c", entry.Run)
	cmd.Dir = entryDir(entry, base)
	cmd.Env = entryEnv(entry)
	cmd.Stdout, cmd.Stderr = w, w
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("starting %s: %w", entry.Name, err)
	}
	pid := cmd.Process.Pid

	// Setsid makes the command the leader of its own group, so the group has
	// the same number as the process.
	boot, _ := state.ProcessBoot(pid)
	if err := state.Write(stateFile, newState(pid, boot)); err != nil {
		// Nothing knows about the process now, so do not leave it behind.
		syscall.Kill(-pid, syscall.SIGKILL)
		return 0, err
	}
	return pid, nil
}

// startBroadcaster starts the broadcaster of one command: another copy of
// runbook, reading the command's output from in and listening on addr for
// whoever wants to hear it. It is not for people to type, so it is not among
// the commands runbook offers.
//
// It gets a session of its own as well, so that stopping the command, which
// signals the command's group, does not reach it before it has passed on the
// last of what the command said.
// It gives back a pipe as well as the process. Nothing reads what the
// broadcaster writes, so its complaints would be lost, and the one that
// matters is why it could not take up its address: start reads that back to
// say what went wrong rather than only that something did.
func startBroadcaster(addr string, in *os.File) (*exec.Cmd, *os.File, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("finding runbook itself: %w", err)
	}
	trouble, said, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("opening a pipe for the broadcaster: %w", err)
	}
	defer said.Close()

	cmd := exec.Command(self, BroadcastCommand, addr)
	cmd.Stdin = in
	cmd.Stderr = said
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		trouble.Close()
		return nil, nil, fmt.Errorf("starting the broadcaster of %s: %w", addr, err)
	}
	return cmd, trouble, nil
}

// whyNot is what the broadcaster said before it gave up, if anything. It is
// only ever read once the broadcaster has gone, so the last of it is there and
// the read ends rather than waiting for more.
func whyNot(trouble *os.File) string {
	said, err := io.ReadAll(io.LimitReader(trouble, 4096))
	if err != nil {
		return ""
	}
	// It is Runbook talking to itself, and it names itself the way it does to
	// anyone else. The message is on its way into another one that has said so
	// already.
	return strings.TrimPrefix(strings.TrimSpace(string(said)), "runbook: ")
}

// waitAddr waits for the broadcaster to take up its address, by connecting to
// it. A broadcaster that never gets there has left the command with nothing to
// write to, so start says so rather than carrying on.
func waitAddr(addr string, wait time.Duration) error {
	const step = 10 * time.Millisecond
	for waited := time.Duration(0); ; waited += step {
		if conn, err := ipc.Dial(addr); err == nil {
			conn.Close()
			return nil
		}
		if waited >= wait {
			return fmt.Errorf("the broadcaster of its output never came up at %s", addr)
		}
		time.Sleep(step)
	}
}

// newState is what start remembers about a command it has just started.
func newState(pid int, boot string) state.State {
	return state.State{PID: pid, Boot: boot, Since: time.Now().Unix()}
}

// stopEntry ends a started command and forgets it, and reports whether it had
// to be killed outright. The signals go to the whole process group, so a
// command that is a shell script takes what it spawned down with it.
func stopEntry(stateFile string, wait time.Duration) (bool, error) {
	st, err := state.Read(stateFile)
	if errors.Is(err, fs.ErrNotExist) {
		return false, errors.New("not running")
	}
	if err != nil {
		return false, err
	}
	if !st.Alive() {
		// The process is long gone; only the file was left behind.
		return false, errors.Join(errors.New("not running"), os.Remove(stateFile))
	}

	if err := syscall.Kill(-st.Group(), syscall.SIGTERM); err != nil {
		return false, fmt.Errorf("stopping %d: %w", st.PID, err)
	}
	killed := false
	if !waitGone(st, wait) {
		if err := syscall.Kill(-st.Group(), syscall.SIGKILL); err != nil {
			return false, fmt.Errorf("killing %d: %w", st.PID, err)
		}
		killed = true
		waitGone(st, wait)
	}
	if err := os.Remove(stateFile); err != nil {
		return killed, err
	}
	return killed, nil
}

// waitGone reports whether the command ended within the time given.
func waitGone(st state.State, wait time.Duration) bool {
	const step = 20 * time.Millisecond
	for waited := time.Duration(0); waited < wait; waited += step {
		if !st.Alive() {
			return true
		}
		time.Sleep(step)
	}
	return !st.Alive()
}

// Start and Stop are what the commands of the same name do: find the entry,
// make sure Runbook has somewhere to keep its files, and report what happened.
func Start(path string, entries []runbookfile.Entry, name string, w io.Writer) error {
	entry, err := runbookfile.Find(entries, name)
	if err != nil {
		return err
	}
	if _, err := workdir.Ensure(path); err != nil {
		return err
	}

	pid, err := startEntry(entry, filepath.Dir(path), state.File(path, entry.Name), ipc.Addr(path, entry.Name))
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "started %s (pid %d)\n", entry.Name, pid)
	return nil
}

func Stop(path string, entries []runbookfile.Entry, name string, w io.Writer) error {
	entry, err := runbookfile.Find(entries, name)
	if err != nil {
		return err
	}

	killed, err := stopEntry(state.File(path, entry.Name), grace)
	if err != nil {
		return fmt.Errorf("%s: %w", entry.Name, err)
	}
	if killed {
		fmt.Fprintf(w, "killed %s, it ignored the request to stop\n", entry.Name)
		return nil
	}
	fmt.Fprintf(w, "stopped %s\n", entry.Name)
	return nil
}
