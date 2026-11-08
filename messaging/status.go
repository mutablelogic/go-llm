package messaging

import (
	"context"
	"strings"
	"sync"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Editor is a provider which can change, and delete, the messages it sent,
// such as to show how a reply is getting on, in a message which is changed
// as it gets on, and deleted once it's replied
type Editor interface {
	// Edit changes the text of the message with the identifier, which was
	// sent to the chat with the address
	Edit(ctx context.Context, address, id, text string) error

	// Delete deletes the message with the identifier, which was sent to the
	// chat with the address
	Delete(ctx context.Context, address, id string) error
}

// status is a message in a chat which shows what's happening as a reply to
// a message from it is written, such as what the tools it calls are doing,
// which is sent once the reply has taken a while, changed as it gets on,
// at most so often, and deleted once it's replied
type status struct {
	ctx      context.Context
	provider Provider
	editor   Editor
	address  string
	delay    time.Duration
	interval time.Duration
	started  time.Time

	mu      sync.Mutex
	line    string    // The latest line, which is shown
	shown   string    // The line which the message shows
	id      string    // The message's identifier, once it's sent
	edited  time.Time // When it was last sent, or changed
	timer   *time.Timer
	sending bool // Whether it's being sent, or changed
	closed  bool
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// statusDelay is how long a reply takes before what's happening is shown,
	// so a reply which is quick shows nothing more
	statusDelay = 3 * time.Second

	// statusInterval is how often the message is changed, at most, as a
	// provider limits how often a bot sends, and changes, messages
	statusInterval = 2 * time.Second

	// statusPrefix is what the message starts with
	statusPrefix = "⏳ "
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newStatus returns the status of a reply in the chat with the address, or
// nil, when the provider can't change its messages, as the message would
// otherwise be left in the chat
func newStatus(ctx context.Context, provider Provider, address string) *status {
	editor, ok := provider.(Editor)
	if !ok {
		return nil
	}
	return &status{ctx: ctx, provider: provider, editor: editor, address: address, delay: statusDelay, interval: statusInterval, started: time.Now()}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// show shows the line, such as what a tool is doing, once the reply has
// taken a while, and at most so often
func (s *status) show(line string) {
	if s == nil {
		return
	}
	line = strings.Join(strings.Fields(line), " ")
	if line == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.line = line
	s.schedule()
}

// schedule sends, or changes, the message, now, or once it's allowed to,
// when it isn't scheduled, or being sent, already. The caller holds the
// lock.
func (s *status) schedule() {
	if s.timer != nil || s.sending {
		return
	}
	now := time.Now()
	at := s.started.Add(s.delay)
	if s.id != "" {
		at = s.edited.Add(s.interval)
	}
	s.timer = time.AfterFunc(max(at.Sub(now), 0), s.flush)
}

// flush sends the message with the latest line, or changes it, when it
// shows another, without the lock held, as it's sent, so showing a line
// doesn't wait for it, and then sends, or changes, it again, when there's a
// later line, or deletes it, when the reply was done as it was sent
func (s *status) flush() {
	s.mu.Lock()
	s.timer = nil
	if s.closed || s.line == "" || s.line == s.shown {
		s.mu.Unlock()
		return
	}
	line, id := s.line, s.id
	s.sending = true
	s.mu.Unlock()

	// Sent, or changed, without the lock held
	text := statusPrefix + line
	sent := id
	var err error
	if id == "" {
		var messages []Sent
		if messages, err = s.provider.Send(s.ctx, s.address, text); err == nil && len(messages) > 0 {
			sent = messages[0].ID
		}
	} else {
		err = s.editor.Edit(s.ctx, s.address, id, text)
	}

	s.mu.Lock()
	s.sending = false
	if err == nil && sent != "" {
		s.id, s.shown, s.edited = sent, line, time.Now()
	}
	orphan := ""
	switch {
	case s.closed:
		// The reply was done as it was sent, so it's deleted, rather than
		// left in the chat
		if id == "" {
			orphan = s.id
		}
	case s.line != s.shown:
		s.schedule()
	}
	s.mu.Unlock()
	if orphan != "" {
		_ = s.editor.Delete(s.ctx, s.address, orphan)
	}
}

// close stops the line being shown, and deletes the message, when it was
// sent, without the lock held, as the reply is done, whether it was
// written, or failed. One which is being sent is deleted once it's sent.
func (s *status) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	id := s.id
	if s.sending {
		// It's being changed, or sent, which deletes it, once it's sent
		if id == "" {
			s.mu.Unlock()
			return
		}
	}
	s.mu.Unlock()
	if id != "" {
		_ = s.editor.Delete(s.ctx, s.address, id)
	}
}
