package registry

import (
	"context"
	"sync"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// statuses are the statuses of the sessions which aren't idle, by their
// identifiers, which are kept by the registry, rather than stored, so
// they're seen by the clients of its server
type statuses struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]*status
}

// status is a session's status: how many replies its model is making, and
// when someone is typing, the timer which makes it idle, unless they say
// they're typing again
type status struct {
	replying int
	typing   *time.Timer
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// TypingTimeout is how long a session is typing, after a client last says
// someone is, before it's idle, so a client which goes away doesn't leave
// it typing
var TypingTimeout = 10 * time.Second

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// SetSessionStatus sets the status of the session with the identifier, as a
// client says someone is typing a message, which it is until TypingTimeout
// after it was last said, or they stopped, when it's idle, unless its model
// is replying, which it is until it has replied. A change of the status is
// published as an update of the session. Returns the session, without its
// messages, with its status. Returns ErrBadParameter when the status isn't
// typing or idle, ErrNotFound when there's no session, and
// ErrServiceUnavailable when the registry has no session store.
func (r *Registry) SetSessionStatus(ctx context.Context, id uuid.UUID, value schema.SessionStatus) (_ *schema.Session, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.SetSessionStatus",
		attribute.String("arg.id", id.String()),
		attribute.String("arg.status", value.String()),
	)
	defer func() { endSpan(err) }()

	if r.sessionStore == nil {
		return nil, errNoSessionStore
	} else if value != schema.SessionTyping && value != schema.SessionIdle {
		return nil, llm.ErrBadParameter.Withf("status %q, which must be typing or idle", value)
	}

	// The session isn't deleted until its status is set
	r.lifecycle.RLock()
	defer r.lifecycle.RUnlock()
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	r.typing(id, value == schema.SessionTyping)
	return r.withoutMessages(session), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// sessionStatus returns the status of the session with the identifier
func (r *Registry) sessionStatus(id uuid.UUID) schema.SessionStatus {
	r.statuses.mu.Lock()
	defer r.statuses.mu.Unlock()
	return r.statuses.sessions[id].value()
}

// typing sets whether someone is typing in the session with the
// identifier, until TypingTimeout from now, and publishes its status, when
// it changes
func (r *Registry) typing(id uuid.UUID, typing bool) {
	r.changeStatus(id, func(s *status) {
		if s.typing != nil {
			s.typing.Stop()
			s.typing = nil
		}
		if !typing {
			return
		}
		var timer *time.Timer
		timer = time.AfterFunc(TypingTimeout, func() {
			r.changeStatus(id, func(s *status) {
				if s.typing == timer {
					s.typing = nil
				}
			})
		})
		s.typing = timer
	})
}

// replying sets the session with the identifier replying, as its model
// replies, and no longer typing, as the message was sent, and publishes its
// status, when it changes. It returns the function which says the model has
// replied, which must be called once.
func (r *Registry) replying(id uuid.UUID) func() {
	r.changeStatus(id, func(s *status) {
		if s.typing != nil {
			s.typing.Stop()
			s.typing = nil
		}
		s.replying++
	})
	var once sync.Once
	return func() {
		once.Do(func() {
			r.changeStatus(id, func(s *status) {
				s.replying--
			})
		})
	}
}

// forgetStatus forgets the status of the session with the identifier, as it
// was deleted, so it isn't published
func (r *Registry) forgetStatus(id uuid.UUID) {
	r.statuses.mu.Lock()
	defer r.statuses.mu.Unlock()
	if s := r.statuses.sessions[id]; s != nil && s.typing != nil {
		s.typing.Stop()
	}
	delete(r.statuses.sessions, id)
}

// changeStatus changes the status of the session with the identifier, with
// the function, and publishes it, when it's changed, under the lock, so the
// events are in the order of the changes. A session which is idle is
// forgotten.
func (r *Registry) changeStatus(id uuid.UUID, fn func(*status)) {
	r.statuses.mu.Lock()
	defer r.statuses.mu.Unlock()
	if r.statuses.sessions == nil {
		r.statuses.sessions = make(map[uuid.UUID]*status)
	}
	s, exists := r.statuses.sessions[id]
	if !exists {
		s = &status{}
	}
	before := s.value()
	fn(s)
	after := s.value()
	if after == schema.SessionIdle {
		delete(r.statuses.sessions, id)
	} else {
		r.statuses.sessions[id] = s
	}
	if before != after {
		r.publish(eventschema.EventUpdate, SessionTable, id.String(), schema.SessionEvent{Status: after})
	}
}

// sessionChanged publishes that the session was created, updated or
// deleted, with its serial, unless it was deleted
func (r *Registry) sessionChanged(action eventschema.EventAction, session schema.Session) {
	var data any
	if action != eventschema.EventDelete {
		data = schema.SessionEvent{Serial: session.Serial}
	}
	r.publish(action, SessionTable, session.ID.String(), data)
}

// withoutMessages returns the session without its messages, which are
// listed on their own, with its status
func (r *Registry) withoutMessages(session schema.Session) *schema.Session {
	session.Messages = nil
	session.Status = r.sessionStatus(session.ID)
	return &session
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// value returns the status, which is replying, while the model is replying,
// typing, while someone is, or idle, which a session without one is
func (s *status) value() schema.SessionStatus {
	switch {
	case s == nil:
		return schema.SessionIdle
	case s.replying > 0:
		return schema.SessionReplying
	case s.typing != nil:
		return schema.SessionTyping
	default:
		return schema.SessionIdle
	}
}
