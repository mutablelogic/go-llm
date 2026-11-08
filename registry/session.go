package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
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
	// Create creates a session with the identifier, or a new one, when it's
	// the nil UUID, with the settings, and no messages, and returns it, with
	// its identifier, serial, and when it was created. Returns ErrConflict
	// when there's a session with the identifier.
	Create(ctx context.Context, id uuid.UUID, meta schema.SessionMeta) (schema.Session, error)

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

	// Watch sends the store's events to the channel, when its sessions are
	// changed by others, such as other servers, from when it returns, until
	// the context is cancelled, as the registry's Watch does: the table is
	// SessionTable, the row the session's identifier, and when it was
	// created or updated, the data is its serial, as a SessionEvent. It
	// returns a channel which is sent nil, or an error when the store stops,
	// once the watch has ended. A store which isn't changed by others, such
	// as files which only its server writes, sends no events.
	Watch(context.Context, chan<- eventschema.Event) (<-chan error, error)
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

	// A session is an event's only when the registry creates it, for an
	// event, and its title is without the space around it, or none, when
	// it's empty
	meta.Event = false
	meta.Title = schema.SessionMeta{}.Merge(schema.SessionMeta{Title: meta.Title}).Title
	if r.sessionStore == nil {
		return nil, errNoSessionStore
	} else if err := validGeneratorMeta(meta.GeneratorMeta); err != nil {
		return nil, err
	} else if meta.Model, err = r.canonicalModel(ctx, meta.Model); err != nil {
		return nil, err
	} else if meta.Tools, err = r.toolNames(ctx, meta.Tools); err != nil {
		return nil, err
	}
	session, err := r.sessionStore.Create(ctx, uuid.Nil(), meta)
	if err != nil {
		return nil, err
	}
	r.sessionChanged(eventschema.EventCreate, session)
	return r.withoutMessages(session), nil
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
	return r.withoutMessages(session), nil
}

// ListSessionMessages returns the messages of the session with the
// identifier, oldest first, as the request selects: of its role, or every
// role, of its context only, which is its last marker, which a summary
// didn't fail for, and the messages after it, when it's asked for, from its
// offset, or, when that's negative, that many from the end, and at most its
// limit of them, and how many there are, with the offset they start at,
// from the start. Returns ErrNotFound when there's no session, and
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
	messages := session.Messages
	if req.Context {
		messages = messages[contextStart(messages):]
	}
	body := make([]providerschema.Message, 0, len(messages))
	for _, message := range messages {
		if req.Role == "" || message.Role() == req.Role {
			body = append(body, message)
		}
	}

	// From the offset, or, when it's negative, that many from the end
	count := len(body)
	offset := req.Offset
	if offset < 0 {
		offset = max(count+offset, 0)
	}
	offset = min(offset, count)
	body = body[offset:]
	if req.Limit != nil && *req.Limit < uint(len(body)) {
		// Compared unsigned, so a limit which is more than an int doesn't
		// overflow
		body = body[:*req.Limit]
	}
	list := &schema.MessageList{MessageListRequest: req, Count: uint(count), Body: body}
	list.Offset = offset
	return list, nil
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
	for i := range list.Body {
		list.Body[i] = *r.withoutMessages(list.Body[i])
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
	previous := session.Tools
	session.SessionMeta = session.SessionMeta.Merge(meta)
	if err := validGeneratorMeta(session.GeneratorMeta); err != nil {
		return nil, err
	}
	if meta.Model != nil {
		if session.Model, err = r.canonicalModel(ctx, session.Model); err != nil {
			return nil, err
		}
	}
	if meta.Tools != nil {
		if session.Tools, err = r.toolNames(ctx, session.Tools, previous...); err != nil {
			return nil, err
		}
	}
	if session, err = r.sessionStore.Put(ctx, session); err != nil {
		return nil, err
	}
	r.sessionChanged(eventschema.EventUpdate, session)
	return r.withoutMessages(session), nil
}

// DeleteSession deletes the session with the identifier, with its messages,
// and returns it, as it was, without them, such as an event's, which is
// created again, when the event runs again. Returns ErrNotFound when
// there's none, ErrConflict when its model is replying, such as when an event
// runs in it, so it's deleted once it's replied, or when it was changed as
// it was deleted, and ErrServiceUnavailable when the registry has no
// session store.
func (r *Registry) DeleteSession(ctx context.Context, id uuid.UUID) (_ *schema.Session, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeleteSession",
		attribute.String("arg.id", id.String()),
	)
	defer func() { endSpan(err) }()

	return r.deleteSession(ctx, id, nil)
}

// deleteSession deletes the session with the identifier, as DeleteSession
// does, and, when the serial isn't nil, only when it's the session's, as it
// was when it was decided it's deleted, and otherwise returns ErrConflict,
// so a session which was saved since isn't deleted
func (r *Registry) deleteSession(ctx context.Context, id uuid.UUID, serial *uint64) (*schema.Session, error) {
	if r.sessionStore == nil {
		return nil, errNoSessionStore
	}

	// A status isn't set as, or after, the session is deleted, and a session
	// which is replying isn't deleted, as what it replies would be lost
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return nil, err
	} else if serial != nil && session.Serial != *serial {
		return nil, llm.ErrConflict.Withf("session %q was changed", id)
	} else if r.sessionStatus(id) == schema.SessionReplying {
		return nil, llm.ErrConflict.Withf("session %q is replying, so it can't be deleted until it has", id)
	} else if err := r.sessionStore.Delete(ctx, id, session.Serial); err != nil {
		return nil, err
	}
	result := r.withoutMessages(session)
	r.forgetStatus(id)
	r.sessionChanged(eventschema.EventDelete, session)
	return result, nil
}

// ForgetSessionMessages forgets the messages of the session with the
// identifier, keeping its settings, so its conversation starts again, and
// returns it, without them. The tokens of its conversation are none, and the
// tokens used by every request to its model are kept. Returns ErrNotFound
// when there's none, ErrConflict when it was changed as they were forgotten,
// and ErrServiceUnavailable when the registry has no session store. A reply
// which is being waited for, as they're forgotten, isn't stored.
func (r *Registry) ForgetSessionMessages(ctx context.Context, id uuid.UUID) (_ *schema.Session, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.ForgetSessionMessages",
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
	session.Messages = nil
	session.Forgotten++
	session.Usage.Input, session.Usage.Output, session.Usage.Overhead = 0, 0, 0
	if session, err = r.sessionStore.Put(ctx, session); err != nil {
		return nil, err
	}
	r.sessionChanged(eventschema.EventUpdate, session)
	return r.withoutMessages(session), nil
}

// Chat adds the messages to the session with the identifier, and the
// messages which its model, asked as WithSession asks it, adds, which are its
// reply, the results of the tools it called, and the rest, and stores it,
// with its usage: the tokens used by every request to its model, and the
// tokens of its conversation, as of the reply. The reply is streamed to the
// function as it's generated, when it isn't nil. Returns what was added,
// with the session's serial, as it was stored. Returns ErrNotFound when
// there's no session, ErrForbidden when it's an event's, which isn't chatted
// in, ErrConflict when it was changed as the model replied,
// as the reply is to the conversation as it was, so isn't stored, apart from
// when messages were only added to it, such as an event's outcome, which was
// delivered, which then follow the reply,
// ErrServiceUnavailable when the registry has no session store, and the
// errors of WithSession. The session is replying until the model has
// replied, and its reply is stored.
func (r *Registry) Chat(ctx context.Context, id uuid.UUID, messages []llm.Message, stream llm.GeneratorStreamFn) (_ *schema.ChatResponse, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Chat",
		attribute.String("arg.id", id.String()),
		attribute.Int("arg.messages", len(messages)),
	)
	defer func() { endSpan(err) }()

	if r.sessionStore == nil {
		return nil, errNoSessionStore
	}
	// The session is replying, until the model has replied, and its reply
	// is stored, or not, unless it's deleted before it's replying
	r.lifecycle.RLock()
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		r.lifecycle.RUnlock()
		return nil, err
	} else if session.Event {
		r.lifecycle.RUnlock()
		return nil, llm.ErrForbidden.Withf("session %q is an event's, which isn't chatted in", id)
	}
	replied := r.replying(id)
	r.lifecycle.RUnlock()
	defer replied()

	// Ask the session's model, and add what it adds to the session, with the
	// usage
	started := time.Now()
	response, err := r.WithSession(ctx, session, messages, stream)
	if err != nil {
		return nil, err
	}
	earlier := session.Messages
	stored, err := r.sessionStore.Put(ctx, withReply(session, earlier, nil, response))

	// Messages which were added to it as it replied, such as an event's
	// outcome, which was delivered, follow the reply, and a marker which was
	// added comes before it, which is stored again, as often as it's changed
	// so, up to so many times, unless it was changed otherwise, such as when
	// its messages were forgotten, or how it's asked was changed, such as its
	// model, or system prompt, though its title can be
	for attempt := 0; errors.Is(err, llm.ErrConflict) && attempt < replyRetries; attempt++ {
		latest, getErr := r.sessionStore.Get(ctx, id)
		if getErr != nil {
			return nil, getErr
		}
		added, ok := addedAfter(earlier, latest.Messages)
		if !ok || latest.Forgotten != session.Forgotten || types.Stringify(latest.GeneratorMeta) != types.Stringify(session.GeneratorMeta) {
			break
		}
		// The earlier messages, as they are now, such as with the summary of
		// a marker, which was written as it replied
		stored, err = r.sessionStore.Put(ctx, withReply(latest, latest.Messages[:len(earlier)], added, response))
	}
	if err != nil {
		return nil, err
	}
	session = stored
	r.sessionChanged(eventschema.EventUpdate, session)

	// The reply, which is the last message, when it's the model's
	if r.replies != nil && len(response.Messages) > 0 {
		if reply := response.Messages[len(response.Messages)-1]; reply.Role() == "assistant" {
			made, err := r.resourcesSince(ctx, session.ID, started, response.Messages)
			if err != nil {
				return nil, err
			}
			r.replies(context.WithoutCancel(ctx), session.ID, Reply{Text: reply.Text(), Resources: resourceLinks(made)})
		}
	}

	// The conversation is compacted, in the background, when it fills most
	// of its model's context, as of the reply
	go r.compactIfFull(context.WithoutCancel(ctx), session, response.Usage)
	return &schema.ChatResponse{ID: session.ID, Serial: session.Serial, SessionResponse: *response}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// withReply returns the session, whose messages are the earlier messages,
// those of the response, which are the question, and its reply, and the
// messages which were added as it replied, with the response's usage. A
// marker which was added as it replied comes before the question, so the
// question, and its reply, are in the context it starts, and other
// messages, such as an event's outcome, follow the reply.
func withReply(session schema.Session, earlier, added []providerschema.Message, response *schema.SessionResponse) schema.Session {
	messages := make([]providerschema.Message, 0, len(earlier)+len(response.Messages)+len(added))
	messages = append(messages, earlier...)
	for _, message := range added {
		if message.Marker != nil {
			messages = append(messages, message)
		}
	}
	messages = append(messages, response.Messages...)
	for _, message := range added {
		if message.Marker == nil {
			messages = append(messages, message)
		}
	}
	session.Messages = messages
	session.Usage = schema.SessionUsage{
		Usage:    addUsage(session.Usage.Usage, response.Usage.Usage),
		Input:    response.Usage.Input,
		Output:   response.Usage.Output,
		Overhead: response.Usage.Overhead,
	}
	return session
}

// addedAfter returns the messages which follow the earlier messages, and
// whether the messages are the earlier messages, exactly as they were,
// apart from the summaries of markers, which are written in the background,
// followed by them, rather than changed otherwise
func addedAfter(earlier, messages []providerschema.Message) ([]providerschema.Message, bool) {
	if len(messages) < len(earlier) {
		return nil, false
	}
	for i := range earlier {
		// A marker, whose summary can be written as a reply is, is the same
		// marker, whatever its summary
		if earlier[i].Marker != nil && messages[i].Marker != nil {
			if earlier[i].Marker.ID != messages[i].Marker.ID {
				return nil, false
			}
			continue
		}
		a, errA := json.Marshal(earlier[i])
		b, errB := json.Marshal(messages[i])
		if errA != nil || errB != nil || !bytes.Equal(a, b) {
			return nil, false
		}
	}
	return messages[len(earlier):], true
}

// replyRetries is how many times a reply is stored again, when its session
// was changed as it was stored, such as when a marker's summary was written
const replyRetries = 5

// errNoSessionStore is the error of a session's method, when the registry
// has no session store
var errNoSessionStore = llm.ErrServiceUnavailable.With("the registry has no session store")

// validGeneratorMeta returns ErrBadParameter when the settings aren't valid:
// their schema isn't a JSON schema, or an option is one which there's a
// field for. They needn't have a model, as it can be inherited, such as from
// another session, or given when an agent's run.
func validGeneratorMeta(meta schema.GeneratorMeta) error {
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
// they are, or none when there are none. A name which the session had
// already, which is kept, is kept as it is, whether it's the toolkit's or
// not, such as a namespace of a connector which was deleted, as its tools
// are skipped as it's chatted in, and may be added again. Returns
// ErrNotFound when another tool isn't the toolkit's, or the registry has no
// toolkit, and ErrConflict when a name is ambiguous, or two of the tools
// have the same name.
func (r *Registry) toolNames(ctx context.Context, names []string, kept ...string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	} else if r.toolkit == nil {
		return nil, llm.ErrNotFound.With("no tools, as the registry has no toolkit")
	}

	// The names which are new are the toolkit's, as its canonical names
	var added []string
	for _, name := range names {
		if !slices.Contains(kept, name) {
			added = append(added, name)
		}
	}
	canonical, err := r.toolkit.ToolNames(ctx, added...)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(names))
	for _, name := range names {
		if slices.Contains(kept, name) {
			result = append(result, name)
		} else {
			result, canonical = append(result, canonical[0]), canonical[1:]
		}
	}

	// And none of them, which are available, have the same name
	if len(added) < len(names) {
		if _, _, err := r.toolkit.AvailableTools(ctx, result...); err != nil {
			return nil, err
		}
	}
	return result, nil
}
