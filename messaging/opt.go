package messaging

import (
	"context"

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
}

// ChatFunc sends the message, which was sent in the channel's chat, to the
// channel's session, and returns once its model has replied, such as by
// chatting in it. The reply is sent to the session's channels by Reply,
// which the session calls with it. It returns ErrNotFound when the session
// doesn't exist, so the channel is disconnected.
type ChatFunc func(ctx context.Context, channel schema.Channel, message Message) error

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
