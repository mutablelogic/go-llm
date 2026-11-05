// Package deadline has a writer of a stream to a client, such as of
// server-sent events, which gives each write its own deadline, rather than
// the server's write timeout, which is for the whole response
package deadline

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Writer writes a stream to a client, with a deadline for each write, so a
// client which stops reading can't block it for longer than the timeout,
// and its Failed channel is closed when a write fails
type Writer struct {
	http.ResponseWriter
	rc      *http.ResponseController
	timeout time.Duration
	once    sync.Once
	failed  chan struct{}
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a writer of a stream to a client, with the timeout of each
// write, and clears the server's deadline of the whole response, unless the
// response writer can't have its deadline set, when it's streamed to until
// the server's timeout
func New(w http.ResponseWriter, timeout time.Duration) (*Writer, error) {
	self := &Writer{ResponseWriter: w, rc: http.NewResponseController(w), timeout: timeout, failed: make(chan struct{})}
	if err := self.rc.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return nil, err
	}
	return self, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Failed returns a channel which is closed when a write fails, as the client
// stopped reading, or went away
func (w *Writer) Failed() <-chan struct{} {
	return w.failed
}

// Write writes to the client, by the deadline
func (w *Writer) Write(data []byte) (int, error) {
	w.deadline()
	n, err := w.ResponseWriter.Write(data)
	if err != nil {
		w.fail()
	}
	return n, err
}

// Flush sends what's written to the client, by the deadline
func (w *Writer) Flush() {
	w.deadline()
	if err := w.rc.Flush(); err != nil {
		w.fail()
	}
}

// Unwrap returns the response writer, for a response controller
func (w *Writer) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// deadline sets the deadline of the next write
func (w *Writer) deadline() {
	if err := w.rc.SetWriteDeadline(time.Now().Add(w.timeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		w.fail()
	}
}

// fail closes the channel which says a write failed
func (w *Writer) fail() {
	w.once.Do(func() { close(w.failed) })
}
