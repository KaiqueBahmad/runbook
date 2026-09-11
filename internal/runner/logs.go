package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"unicode/utf8"

	"runbook/internal/ipc"
	"runbook/internal/runbookfile"
	"runbook/internal/workdir"
)

// Logs shows what a started command writes, from now on. It connects to the
// broadcaster the command's output goes to and copies what comes back, until
// the command ends and the broadcaster closes the connection, or until whoever
// asked for it presses Ctrl-C.
//
// There is no history behind it. Runbook writes nothing down, so what the
// command said before this connected was heard by whoever was listening then,
// and by nobody else.
func Logs(path string, entries []runbookfile.Entry, name string, w io.Writer) error {
	entry, err := runbookfile.Find(entries, name)
	if err != nil {
		return err
	}

	work, err := workdir.Path(path)
	if err != nil {
		return err
	}

	conn, err := ipc.Dial(ipc.Addr(work, entry.Name))
	if err != nil {
		return fmt.Errorf("nothing is broadcasting %s, run 'runbook start %s' first", entry.Name, entry.Name)
	}
	defer conn.Close()

	if _, err := io.Copy(w, conn); err != nil {
		return fmt.Errorf("listening to %s: %w", entry.Name, err)
	}
	return nil
}

// LogsAll shows what every started command of a runbook.yml writes, from now
// on, with the name of the command in front of each line so that one output is
// told from another. It connects to every command broadcasting at the moment it
// is asked, and returns once the last of them has ended, or when whoever asked
// for it presses Ctrl-C. A command started after it began is not picked up: the
// commands are those that were running when it looked.
//
// Nothing is behind it either. What the commands said before this connected is
// gone, the same way it is for one command.
//
// align lines the names up in a column, for a person reading; without it a tab
// separates the name from the line, for something else to take apart.
func LogsAll(path string, entries []runbookfile.Entry, w io.Writer, align bool) error {
	work, err := workdir.Path(path)
	if err != nil {
		return err
	}

	type listening struct {
		name string
		conn net.Conn
	}

	var on []listening
	for _, entry := range entries {
		conn, err := ipc.Dial(ipc.Addr(work, entry.Name))
		if err != nil {
			// The command is not running, which is not something to say: what
			// was asked for is everything that is.
			continue
		}
		on = append(on, listening{name: entry.Name, conn: conn})
	}
	if len(on) == 0 {
		return errors.New("nothing is broadcasting, run 'runbook start <name>' first")
	}

	width := 0
	if align {
		for _, l := range on {
			width = max(width, utf8.RuneCountInString(l.name))
		}
	}

	out := &syncWriter{w: w}
	errs := make([]error, len(on))
	var wg sync.WaitGroup
	for i, l := range on {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer l.conn.Close()

			p := &prefixer{out: out, prefix: label(l.name, width)}
			if _, err := io.Copy(p, l.conn); err != nil {
				errs[i] = fmt.Errorf("listening to %s: %w", l.name, err)
			}
			p.flush()
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

// label is what stands in front of a line to say which command wrote it. A
// width lines the names up in a column; without one a tab separates them, so
// that a name with spaces in it is still one field.
func label(name string, width int) string {
	if width == 0 {
		return name + "\t"
	}
	return name + strings.Repeat(" ", width-utf8.RuneCountInString(name)) + "  "
}

// syncWriter is the one output several commands are written to at once, a whole
// line at a time, so that no line is written into the middle of another.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// line writes one command's line, and says nothing of an output that has gone:
// there is nowhere left to report it to.
func (s *syncWriter) line(prefix string, line []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.w, "%s%s\n", prefix, line)
}

// prefixer takes what one command writes and passes on its finished lines with
// the command's name in front of each.
//
// It holds a line back until the newline behind it arrives. That is the price
// of several commands sharing one output: a line half written by one would
// otherwise be run into by the next line of another, and neither could be read.
type prefixer struct {
	out    *syncWriter
	prefix string
	buf    []byte
}

func (p *prefixer) Write(b []byte) (int, error) {
	n := len(b)
	for {
		end := bytes.IndexByte(b, '\n')
		if end < 0 {
			p.buf = append(p.buf, b...)
			break
		}
		p.buf = append(p.buf, b[:end]...)
		p.line()
		b = b[end+1:]
	}
	return n, nil
}

// flush passes on what the command left without a newline behind it, now that
// there is no more coming.
func (p *prefixer) flush() {
	if len(p.buf) > 0 {
		p.line()
	}
}

func (p *prefixer) line() {
	p.out.line(p.prefix, p.buf)
	p.buf = p.buf[:0]
}
