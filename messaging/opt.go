package messaging

import (
	"context"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	schema "github.com/mutablelogic/go-llm/messaging/schema"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of messaging
type Opt func(*opt) error

type opt struct {
	providers   []Provider
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
	chat        ChatFunc
	session     SessionFunc
	open        OpenFunc
}

// ChatFunc sends the message, which was sent in the channel's chat, to the
// channel's session, and returns once its model has replied, such as by
// chatting in it. The reply is sent to the session's channels by Reply,
// which the session calls with it.
type ChatFunc func(ctx context.Context, channel schema.Channel, message Message) error

// SessionFunc returns nil when the session with the identifier exists, or
// ErrNotFound when it doesn't
type SessionFunc func(context.Context, uuid.UUID) error

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opt) apply(opts []Opt) error {
	for _, fn := range opts {
		if err := fn(o); err != nil {
			return err
		}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithProvider adds the messaging provider, such as Telegram, whose chats
// can be connected. Returns ErrBadParameter when it's nil, and ErrConflict
// when one with its name was added.
func WithProvider(provider Provider) Opt {
	return func(o *opt) error {
		if provider == nil {
			return llm.ErrBadParameter.With("missing messaging provider")
		}
		for _, other := range o.providers {
			if other.Name() == provider.Name() {
				return llm.ErrConflict.Withf("messaging provider %q added twice", provider.Name())
			}
		}
		o.providers = append(o.providers, provider)
		return nil
	}
}

// WithBroadcaster sets the broadcaster which Run watches for the sessions
// which are deleted, whose channels are disconnected
func WithBroadcaster(b *broadcaster.Broadcaster) Opt {
	return func(o *opt) error {
		if b == nil {
			return llm.ErrBadParameter.With("missing broadcaster")
		}
		o.broadcaster = b
		return nil
	}
}

// WithTracer sets the tracer which traces what's sent, and received, or
// none, when it's nil
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}

// WithChat sets the function which sends a message from a channel to its
// session, without which the messages are ignored
func WithChat(fn ChatFunc) Opt {
	return func(o *opt) error {
		o.chat = fn
		return nil
	}
}

// WithSessions sets the function which says whether a session exists, so a
// chat isn't connected to a session which was deleted, and a channel whose
// session was deleted, such as when the event that it was deleted was
// missed, is disconnected, when a message from it can't be answered
func WithSessions(fn SessionFunc) Opt {
	return func(o *opt) error {
		o.session = fn
		return nil
	}
}

// WithFiles sets the function which opens a session's files, so they're
// sent to its channels, with its replies, by the providers which can send
// files, without which only text is sent
func WithFiles(fn OpenFunc) Opt {
	return func(o *opt) error {
		o.open = fn
		return nil
	}
}
