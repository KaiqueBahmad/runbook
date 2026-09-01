package ipc

import (
	"io"
	"net"
	"sync"
	"time"
)

// A started command writes into a pipe rather than to a file: Runbook keeps no
// logs, and so has no place that fills up and nothing to trim. At the far end
// of the pipe is a broadcaster, a second copy of runbook that reads everything
// the command says and passes it on to whoever is listening at that moment.
//
// What that costs is the past. A listener hears the command from the moment it
// connects, and what was said before then, or while nobody was there, is gone.

// backlog is how far behind one listener may fall, in chunks of output, before
// the broadcaster lets go of it. It is deep enough that a terminal keeping up
// never notices, and shallow enough that one that has stopped reading is not
// carried for long.
const backlog = 256

// flush is how long the listeners have to write out the last of the output
// once the command has ended.
const flush = time.Second

// broadcaster passes what one command writes to everyone listening to it.
//
// The chunks go to a channel per listener, each with a writer of its own, so
// the one thing that must not stop, reading the command's output, never waits
// on a terminal that is slow, paused, or gone.
type broadcaster struct {
	mu        sync.Mutex
	listeners map[chan []byte]struct{}
	over      bool           // the command has ended, so there is no more to hear
	writers   sync.WaitGroup // the writers still to finish
}

func newBroadcaster() *broadcaster {
	return &broadcaster{listeners: map[chan []byte]struct{}{}}
}

// Broadcast reads a command's output from in and hands it to whoever connects
// to addr, until the command ends.
func Broadcast(addr string, in io.Reader) error {
	l, err := Listen(addr)
	if err != nil {
		return err
	}
	defer l.Close()

	b := newBroadcaster()
	go b.accept(l)

	b.drain(in)
	b.end()
	return nil
}

// drain reads the command's output until it ends. It takes whatever is there
// rather than whole lines, so that a command asking a question without a
// newline behind it still reaches the terminals listening.
//
// It never waits on a listener, which is the point of it: a command whose
// output nobody reads must not stop writing.
func (b *broadcaster) drain(in io.Reader) {
	buf := make([]byte, 32*1024)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			// The buffer is read into again, so what is passed on is a copy.
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			b.send(chunk)
		}
		if err != nil {
			return
		}
	}
}

// send offers a chunk to every listener. One whose backlog is full has stopped
// reading, and is let go of rather than waited for.
func (b *broadcaster) send(chunk []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.listeners {
		select {
		case ch <- chunk:
		default:
			delete(b.listeners, ch)
			close(ch)
		}
	}
}

// accept takes on the listeners as they connect.
func (b *broadcaster) accept(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			// The listener is closed, which happens when the command has
			// ended and there is nothing left to listen to.
			return
		}
		b.add(conn)
	}
}

// add takes on one listener, unless the command has already ended.
func (b *broadcaster) add(conn net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.over {
		conn.Close()
		return
	}
	ch := make(chan []byte, backlog)
	b.listeners[ch] = struct{}{}
	b.writers.Add(1)
	go b.write(conn, ch)

	// A listener says nothing of its own, so a read that ends is a listener
	// that has gone. Noticing it here rather than at the next thing the command
	// writes is what keeps a command that has fallen silent from carrying the
	// terminals that have long since closed.
	go func() {
		io.Copy(io.Discard, conn)
		b.drop(ch)
	}()
}

// write is the whole of what one listener costs the others: its own goroutine,
// waiting on nothing but itself.
func (b *broadcaster) write(conn net.Conn, ch chan []byte) {
	defer b.writers.Done()
	defer conn.Close()

	for chunk := range ch {
		if _, err := conn.Write(chunk); err != nil {
			// The terminal at the other end has gone away.
			b.drop(ch)
			return
		}
	}
}

// drop lets go of one listener. Only a listener still in the map is closed, and
// only while the lock is held, so one that leaves just as the command writes is
// closed once and not twice.
func (b *broadcaster) drop(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.listeners[ch]; ok {
		delete(b.listeners, ch)
		close(ch)
	}
}

// end closes the listeners now that the command has ended, and gives them a
// moment to write out the last of what it said. Each of them sees the
// connection close, which is how runbook logs knows to stop and return.
func (b *broadcaster) end() {
	b.mu.Lock()
	b.over = true
	for ch := range b.listeners {
		delete(b.listeners, ch)
		close(ch)
	}
	b.mu.Unlock()

	done := make(chan struct{})
	go func() {
		b.writers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(flush):
		// A listener that cannot take the last of the output in a second is
		// not worth holding the broadcaster open for.
	}
}
