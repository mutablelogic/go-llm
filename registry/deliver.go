package registry

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// deliverWait is how long an event's outcome waits for the session it's
	// for to stop replying, before it's added anyway, which a chat's reply,
	// stored after it, is added after
	deliverWait = 30 * time.Second

	// deliverPoll is how often it looks whether the session is replying
	deliverPoll = 500 * time.Millisecond

	// deliverRetries is how many times it's added again, when the session
	// changed as it was, such as when a chat's reply was stored
	deliverRetries = 5
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// deliver adds the event's outcome, which is to be told to the person, to
// the session it's for, as the assistant's message, marked as the event's,
// with the resources its run made, such as speech, copied to the session,
// once the session isn't replying, so a chat's reply isn't refused, as its
// session changed, and marks it delivered, and gives it to the replies
// function, such as to send it to the session's channels, as a reply is.
// It's called in a goroutine of its own, so the run doesn't wait for it.
func (r *Registry) deliver(ctx context.Context, event schema.Event, run []providerschema.Message) (err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeliverEvent",
		attribute.String("arg.event", event.ID.String()),
		attribute.String("arg.session", event.Session.String()),
	)
	defer func() { endSpan(err) }()

	text := strings.TrimSpace(event.Message)
	if event.Status != schema.EventPending || text == "" || event.Session == uuid.Nil() || r.sessionStore == nil {
		return nil
	}
	message := providerschema.Message{
		MessageMeta: providerschema.MessageMeta{
			Sender:  "assistant",
			Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}},
		},
		Result: providerschema.ResultStop,
		Event:  &providerschema.Event{ID: event.ID, Title: event.Title},
	}

	// The resources the run made are the session's too, so the message can
	// have them
	if r.resources != nil && event.Run != uuid.Nil() {
		for _, link := range madeResources(run) {
			copied, err := r.resources.Copy(ctx, link.URI, event.Run, event.Session)
			if err != nil {
				return err
			}
			u, err := url.Parse(copied.URI())
			if err != nil {
				return err
			}
			message.Content = append(message.Content, providerschema.ContentBlock{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: copied.Type, URL: u}}})
		}
	}

	// Wait for the session to stop replying, and add it, again, when the
	// session changed as it was added
	for attempt := 0; ; attempt++ {
		if err := r.waitIdle(ctx, event.Session); err != nil {
			return err
		}
		err = r.appendMessage(ctx, event.Session, message)
		if !errors.Is(err, llm.ErrConflict) || attempt >= deliverRetries {
			break
		}
	}
	if err != nil {
		return err
	}

	// It's delivered, and the session's channels, and the like, are given
	// it, as they're given a reply
	if _, err := r.updateEvent(ctx, event.ID, func(e *schema.Event) {
		e.Status = schema.EventDelivered
	}); err != nil {
		return err
	}
	if r.replies != nil {
		r.replies(ctx, event.Session, text)
	}
	return nil
}

// waitIdle waits until the session isn't replying, or for deliverWait, at
// most, or until the context is cancelled, when it returns its error
func (r *Registry) waitIdle(ctx context.Context, id uuid.UUID) error {
	deadline := time.Now().Add(deliverWait)
	for r.sessionStatus(id) == schema.SessionReplying && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(deliverPoll):
		}
	}
	return nil
}

// appendMessage adds the message to the session, with the time now, and
// stores it, and publishes that it changed. Returns ErrNotFound when
// there's no session, and ErrConflict when it changed as it was added.
func (r *Registry) appendMessage(ctx context.Context, id uuid.UUID, message providerschema.Message) error {
	r.lifecycle.RLock()
	defer r.lifecycle.RUnlock()
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return err
	}
	message.Time = time.Now().UTC()
	session.Messages = append(session.Messages, message)
	if session, err = r.sessionStore.Put(ctx, session); err != nil {
		return err
	}
	r.sessionChanged(eventschema.EventUpdate, session)
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// madeResources returns the resources which the messages of a run made, as
// links, in the order they were made, once each: those the tools it called
// returned, and those in its replies
func madeResources(messages []providerschema.Message) []toolschema.ResourceLink {
	var result []toolschema.ResourceLink
	seen := make(map[string]bool)
	add := func(link toolschema.ResourceLink) {
		hash, err := resourceschema.ParseURI(link.URI)
		if err != nil || seen[hash] {
			return
		}
		seen[hash] = true
		result = append(result, link)
	}
	for _, message := range messages {
		for _, block := range message.Content {
			switch {
			case block.ToolResult != nil && !block.ToolResult.IsError:
				var returned toolschema.ResourcesResult
				if err := json.Unmarshal(block.ToolResult.Content, &returned); err == nil {
					for _, link := range returned.Resources {
						add(link)
					}
				}
			case block.Resource != nil && block.Resource.URL != nil && message.Role() == "assistant":
				add(toolschema.ResourceLink{URI: block.Resource.URL.String(), Type: block.Resource.ContentType})
			}
		}
	}
	return result
}
