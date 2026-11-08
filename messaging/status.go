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

	mu     sync.Mutex
	line   string    // The latest line, which is shown
	shown  string    // The line which the message shows
	id     string    // The message's identifier, once it's sent
	edited time.Time // When it was last sent, or changed
	timer  *time.Timer
	closed bool
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
// when it isn't scheduled already. The caller holds the lock.
func (s *status) schedule() {
	if s.timer != nil {
		return
	}
	now := time.Now()
	at := s.started.Add(s.delay)
	if s.id != "" {
		at = s.edited.Add(s.interval)
	}
	if !at.After(now) {
		s.flush()
		return
	}
	s.timer = time.AfterFunc(at.Sub(now), func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.timer = nil
		if !s.closed {
			s.flush()
		}
	})
}

// flush sends the message with the latest line, or changes it, when it
// shows another. The caller holds the lock.
func (s *status) flush() {
	if s.line == "" || s.line == s.shown {
		return
	}
	text := statusPrefix + s.line
	if s.id == "" {
		sent, err := s.provider.Send(s.ctx, s.address, text)
		if err != nil || len(sent) == 0 {
			return
		}
		s.id = sent[0].ID
	} else if err := s.editor.Edit(s.ctx, s.address, s.id, text); err != nil {
		return
	}
	s.shown, s.edited = s.line, time.Now()
}

// close stops the line being shown, and deletes the message, when it was
// sent, as the reply is done, whether it was written, or failed
func (s *status) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if s.id != "" {
		_ = s.editor.Delete(s.ctx, s.address, s.id)
	}
}
