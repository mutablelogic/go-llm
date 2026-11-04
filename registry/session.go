package registry

import (
	"context"
	"strings"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// SessionStore has sessions, each with its identifier and serial, which the
// store sets, and changes each time the session is stored, so a change is
// made to the session as it was last seen, rather than a newer one, such as
// when two replies are added to it at once
type SessionStore interface {
	// Create creates a session with the settings, and no messages, and
	// returns it, with its identifier, serial, and when it was created
	Create(context.Context, schema.SessionMeta) (schema.Session, error)

	// Get returns the session with the identifier, with its messages.
	// Returns ErrNotFound when there's none.
	Get(ctx context.Context, id uuid.UUID) (schema.Session, error)

	// List returns the sessions, without their messages, most recently
	// modified first, from the request's offset, and at most its limit of
	// them, and how many there are
	List(context.Context, schema.SessionListRequest) (schema.SessionList, error)

	// Put replaces the session with its identifier, when its serial is that
	// session's, and returns it, with its new serial, and when it was
	// modified. Returns ErrNotFound when there's none, and ErrConflict when
	// its serial isn't the session's, as it was changed since it was seen.
	Put(context.Context, schema.Session) (schema.Session, error)

	// Delete deletes the session with the identifier, when the serial is its
	// serial, or does nothing when there's none. Returns ErrConflict when the
	// serial isn't its serial, as it was changed since it was seen.
	Delete(ctx context.Context, id uuid.UUID, serial uint64) error
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateSession creates a session with the settings, in the registry's
// session store, and returns it, with no messages. Its model, when it has
// one, is the registry's, by its canonical name, such as
// ollama:llama3.2:latest, and its tools are the toolkit's, by their canonical
// names, such as weatherapi:current, or a namespace and "*", or "*", as they
// are. It needn't have a model, as it can be inherited. Returns
// ErrBadParameter when the settings aren't valid, ErrNotFound when the model
// isn't the registry's, or a tool isn't the toolkit's, ErrConflict when a
// name is ambiguous, and ErrServiceUnavailable when the registry has no
// session store.
func (r *Registry) CreateSession(ctx context.Context, meta schema.SessionMeta) (_ *schema.Session, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.CreateSession",
		attribute.String("arg.meta", meta.String()),
	)
	defer func() { endSpan(err) }()

	if r.sessionStore == nil {
		return nil, errNoSessionStore
	} else if err := validSessionMeta(meta); err != nil {
		return nil, err
	} else if meta.Model, err = r.canonicalModel(ctx, meta.Model); err != nil {
		return nil, err
	} else if meta.Tools, err = r.toolNames(ctx, meta.Tools); err != nil {
		return nil, err
	}
	session, err := r.sessionStore.Create(ctx, meta)
	if err != nil {
		return nil, err
	}
	return withoutMessages(session), nil
}

// GetSession returns the session with the identifier, without its
// messages, which ListSessionMessages lists.
// Returns ErrNotFound when there's none, and ErrServiceUnavailable when
// the registry has no session store.
func (r *Registry) GetSession(ctx context.Context, id uuid.UUID) (_ *schema.Session, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.GetSession",
		attribute.String("arg.id", id.String()),
	)
	defer func() { endSpan(err) }()

	if r.sessionStore == nil {
		return nil, errNoSessionStore
	}
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return withoutMessages(session), nil
}

// ListSessionMessages returns the messages of the session with the
// identifier, oldest first, as the request selects: of its role, or every
// role, from its offset, and at most its limit of them, and how many there
// are. Returns ErrNotFound when there's no session, and
// ErrServiceUnavailable when the registry has no session store.
func (r *Registry) ListSessionMessages(ctx context.Context, id uuid.UUID, req schema.MessageListRequest) (_ *schema.MessageList, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListSessionMessages",
		attribute.String("arg.id", id.String()),
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	if r.sessionStore == nil {
		return nil, errNoSessionStore
	}
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	body := make([]providerschema.Message, 0, len(session.Messages))
	for _, message := range session.Messages {
		if req.Role == "" || message.Role() == req.Role {
			body = append(body, message)
		}
	}
	count := uint(len(body))
	body = body[min(req.Offset, count):]
	if req.Limit != nil {
		body = body[:min(*req.Limit, uint(len(body)))]
	}
	return &schema.MessageList{MessageListRequest: req, Count: count, Body: body}, nil
}

// ListSessions returns the sessions, without their messages, most recently
// modified first, as the request selects. Returns ErrServiceUnavailable when
// the registry has no session store.
func (r *Registry) ListSessions(ctx context.Context, req schema.SessionListRequest) (_ *schema.SessionList, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ListSessions",
		attribute.String("arg.req", req.String()),
	)
	defer func() { endSpan(err) }()

	if r.sessionStore == nil {
		return nil, errNoSessionStore
	}
	list, err := r.sessionStore.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &list, nil
}

// UpdateSession changes the settings of the session with the identifier, as
// SessionMeta.Merge does: what the update sets is changed, its options are
// set, or removed, when they're null, and the rest is kept, as are the
// session's messages. A model, or tools, which are changed are the
// registry's, by their canonical names, as CreateSession says. Returns the
// session. Returns ErrBadParameter when the update changes nothing, or the
// settings, as changed, aren't valid, ErrNotFound when there's no session,
// or the model, or a tool, isn't the registry's,
// ErrConflict when it was changed as it was updated, and
// ErrServiceUnavailable when the registry has no session store.
func (r *Registry) UpdateSession(ctx context.Context, id uuid.UUID, meta schema.SessionMeta) (_ *schema.Session, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.UpdateSession",
		attribute.String("arg.id", id.String()),
		attribute.String("arg.meta", meta.String()),
	)
	defer func() { endSpan(err) }()

	if r.sessionStore == nil {
		return nil, errNoSessionStore
	} else if meta.IsZero() {
		return nil, llm.ErrBadParameter.With("nothing to update")
	}
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	session.SessionMeta = session.SessionMeta.Merge(meta)
	if err := validSessionMeta(session.SessionMeta); err != nil {
		return nil, err
	}
	if meta.Model != nil {
		if session.Model, err = r.canonicalModel(ctx, session.Model); err != nil {
			return nil, err
		}
	}
	if meta.Tools != nil {
		if session.Tools, err = r.toolNames(ctx, session.Tools); err != nil {
			return nil, err
		}
	}
	if session, err = r.sessionStore.Put(ctx, session); err != nil {
		return nil, err
	}
	return withoutMessages(session), nil
}

// DeleteSession deletes the session with the identifier, with its messages,
// and returns it, as it was, without them. Returns ErrNotFound when there's none, ErrConflict when it was
// changed as it was deleted, and ErrServiceUnavailable when the registry has
// no session store.
func (r *Registry) DeleteSession(ctx context.Context, id uuid.UUID) (_ *schema.Session, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeleteSession",
		attribute.String("arg.id", id.String()),
	)
	defer func() { endSpan(err) }()

	if r.sessionStore == nil {
		return nil, errNoSessionStore
	}
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return nil, err
	} else if err := r.sessionStore.Delete(ctx, id, session.Serial); err != nil {
		return nil, err
	}
	return withoutMessages(session), nil
}

// Chat adds the messages to the session with the identifier, and the
// messages which its model, asked as WithSession asks it, adds, which are its
// reply, the results of the tools it called, and the rest, and stores it,
// with its usage: the tokens used by every request to its model, and the
// tokens of its conversation, as of the reply. The reply is streamed to the
// function as it's generated, when it isn't nil. Returns what was added,
// with the session's serial, as it was stored. Returns ErrNotFound when
// there's no session, ErrConflict when it was changed as the model replied,
// as the reply is to the conversation as it was, so isn't stored,
// ErrServiceUnavailable when the registry has no session store, and the
// errors of WithSession.
func (r *Registry) Chat(ctx context.Context, id uuid.UUID, messages []llm.Message, stream llm.GeneratorStreamFn) (_ *schema.ChatResponse, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Chat",
		attribute.String("arg.id", id.String()),
		attribute.Int("arg.messages", len(messages)),
	)
	defer func() { endSpan(err) }()

	if r.sessionStore == nil {
		return nil, errNoSessionStore
	}
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Ask the session's model, and add what it adds to the session, with the
	// usage
	response, err := r.WithSession(ctx, session, messages, stream)
	if err != nil {
		return nil, err
	}
	session.Messages = append(session.Messages, response.Messages...)
	session.Usage = schema.SessionUsage{
		Usage:    addUsage(session.Usage.Usage, response.Usage.Usage),
		Input:    response.Usage.Input,
		Output:   response.Usage.Output,
		Overhead: response.Usage.Overhead,
	}
	if session, err = r.sessionStore.Put(ctx, session); err != nil {
		return nil, err
	}
	return &schema.ChatResponse{ID: session.ID, Serial: session.Serial, SessionResponse: *response}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// errNoSessionStore is the error of a session's method, when the registry
// has no session store
var errNoSessionStore = llm.ErrServiceUnavailable.With("the registry has no session store")

// withoutMessages returns the session without its messages, which are
// listed on their own
func withoutMessages(session schema.Session) *schema.Session {
	session.Messages = nil
	return &session
}

// validSessionMeta returns ErrBadParameter when the settings aren't valid:
// their schema isn't a JSON schema, or an option is one which there's a
// field for. They needn't have a model, as it can be inherited, such as from
// another session.
func validSessionMeta(meta schema.SessionMeta) error {
	if _, err := meta.JSONSchema(); err != nil {
		return err
	} else if _, err := meta.Opts(types.Value(meta.System)); err != nil {
		return err
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// canonicalModel returns the model's canonical name, such as
// ollama:llama3.2:latest, when it has a name, or nil. Returns ErrNotFound
// when there's no model with the name, and ErrConflict when it's ambiguous.
func (r *Registry) canonicalModel(ctx context.Context, name *string) (*string, error) {
	if name == nil || strings.TrimSpace(*name) == "" {
		return nil, nil
	}
	model, err := r.GetModel(ctx, strings.TrimSpace(*name))
	if err != nil {
		return nil, err
	}
	return types.Ptr(r.Name(model)), nil
}

// toolNames returns the names of the tools, as the toolkit's canonical names,
// such as weatherapi:current for current, or a namespace and "*", or "*", as
// they are, or none when there are none. Returns ErrNotFound when a tool
// isn't the toolkit's, or the registry has no toolkit, and ErrConflict when
// a name is ambiguous.
func (r *Registry) toolNames(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	} else if r.toolkit == nil {
		return nil, llm.ErrNotFound.With("no tools, as the registry has no toolkit")
	}
	return r.toolkit.ToolNames(ctx, names...)
}
