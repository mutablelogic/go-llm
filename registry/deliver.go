package registry

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// delivery is how events' outcomes are delivered: how long the session
// they're for must have been idle first, and how long they wait for it
type delivery struct {
	// idle is how long the session must have been idle, without a reply,
	// someone typing, or a message, before a notification is added, so it
	// doesn't interrupt a conversation
	idle time.Duration

	// maxWait is how long a notification waits, at most, for the session
	// to be idle, before it's added anyway, which a chat's reply, stored
	// after it, is added after
	maxWait time.Duration

	// alertWait is how long an alert, which is added once the session is
	// idle, without waiting for it to have been for a while, waits for it,
	// at most, as acting late would cost the person something
	alertWait time.Duration

	// poll is how often it looks whether the session is idle
	poll time.Duration

	// retries is how many times it's added again, when the session changed
	// as it was, such as when a chat's reply was stored
	retries int

	// retryEvery is how often the outcomes which are pending, which weren't
	// delivered, are, and retain how long after they last changed they're
	// tried
	retryEvery time.Duration
	retain     time.Duration
}

// defaultDelivery is how events' outcomes are delivered
var defaultDelivery = delivery{
	idle:       30 * time.Second,
	maxWait:    10 * time.Minute,
	alertWait:  30 * time.Second,
	poll:       500 * time.Millisecond,
	retries:    5,
	retryEvery: time.Minute,
	retain:     24 * time.Hour,
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// deliver adds the event's outcome, which is to be told to the person, to
// the session it's for, as the assistant's message, marked as the event's,
// with the resources its run made since it started, such as speech, copied
// to the session, once the session has been idle for a while, or, for an
// alert, once it's idle, so it doesn't interrupt a conversation, and gives
// it to the replies function, such as to send it to the session's channels,
// as a reply is, and marks it delivered. An outcome which is being
// delivered isn't delivered again, at the same time, and one whose message
// the session has already isn't added again, so a delivery which failed can
// be tried again. It's called in a goroutine of its own, so the run doesn't
// wait for it.
func (r *Registry) deliver(ctx context.Context, event schema.Event, since time.Time) (err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeliverEvent",
		attribute.String("arg.event", event.ID.String()),
		attribute.String("arg.session", event.Session.String()),
	)
	defer func() { endSpan(err) }()

	text := strings.TrimSpace(event.Message)
	if event.Status != schema.EventPending || text == "" || event.Session == uuid.Nil() || r.sessionStore == nil {
		return nil
	}
	if _, delivering := r.delivering.LoadOrStore(event.ID, struct{}{}); delivering {
		return nil
	}
	defer r.delivering.Delete(event.ID)

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
	made, err := r.copyRunResources(ctx, event, since)
	if err != nil {
		return err
	}
	for _, resource := range made {
		u, err := url.Parse(resource.URI())
		if err != nil {
			return err
		}
		message.Content = append(message.Content, providerschema.ContentBlock{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: resource.Type, URL: u}}})
	}

	// What the system says of it is the task which sent it, and why it ran,
	// so the model can tell it from its replies to the person
	message.Content = append(message.Content, providerschema.NewAnnotation(eventOrigin(event)))

	// Wait for the session to be idle, and add it, again, when the session
	// changed as it was added
	idle, maxWait := r.delivery.idle, r.delivery.maxWait
	if event.Importance == schema.ImportanceAlert {
		idle, maxWait = 0, r.delivery.alertWait
	}
	added := false
	for attempt := 0; ; attempt++ {
		if err := r.waitIdle(ctx, event.Session, idle, maxWait); err != nil {
			return err
		}
		added, err = r.appendMessage(ctx, event.Session, message)
		if !errors.Is(err, llm.ErrConflict) || attempt >= r.delivery.retries {
			break
		}
	}
	if err != nil {
		return err
	}

	// The session's channels, and the like, are given it, with the files
	// its run made, once it's added, as they're given a reply, and it's
	// delivered
	if added && r.replies != nil {
		r.replies(ctx, event.Session, Reply{Text: text, Resources: resourceLinks(made)})
	}
	_, err = r.updateEvent(ctx, event.ID, func(e *schema.Event) {
		e.Status = schema.EventDelivered
	})
	return err
}

// eventOrigin returns what the system says of the event's message, which is
// the task which sent it, and why it ran, when that's known: on schedule, as
// a heartbeat, or as its source triggered it
func eventOrigin(event schema.Event) string {
	task := "a task"
	if title := strings.TrimSpace(event.Title); title != "" {
		task = fmt.Sprintf("the task %q", title)
	}
	switch source := strings.TrimSpace(event.Source); {
	case source != "":
		return fmt.Sprintf("Sent by %s, which %s triggered.", task, source)
	case event.Heartbeat:
		return "Sent by " + task + ", which ran on schedule."
	default:
		return "Sent by " + task + "."
	}
}

// deliverInBackground delivers the event's outcome, in a goroutine of its
// own, logging why, when it isn't, so it's tried again
func (r *Registry) deliverInBackground(ctx context.Context, event schema.Event, since time.Time) {
	go func() {
		if err := r.deliver(context.WithoutCancel(ctx), event, since); err != nil {
			r.log().WarnContext(ctx, "event not delivered, which is tried again", "event", event.ID, "session", event.Session, "error", err.Error())
		}
	}()
}

// retryDeliveries delivers the outcomes which are pending, which weren't,
// such as when their delivery failed, or the server stopped, which changed
// within deliveryRetain, and aren't being delivered
func (r *Registry) retryDeliveries(ctx context.Context) {
	if r.eventStore == nil {
		return
	}
	var pending []schema.Event
	limit := uint(100)
	for offset := uint(0); ; offset += limit {
		page, err := r.ListEvents(ctx, schema.EventListRequest{Status: schema.EventPending, Offset: offset, Limit: &limit})
		if err != nil {
			r.log().ErrorContext(ctx, "pending events not listed", "error", err.Error())
			return
		}
		pending = append(pending, page.Body...)
		if uint(len(page.Body)) < limit {
			break
		}
	}
	for _, event := range pending {
		if time.Since(event.Modified) > r.delivery.retain {
			continue
		} else if _, delivering := r.delivering.Load(event.ID); delivering {
			continue
		}
		r.deliverInBackground(ctx, event, runStarted(event))
	}
}

// waitIdle waits until the session has been idle, neither replying, nor
// with someone typing, for the time, since its last message, or since it
// last wasn't idle, or until it's waited for the most time, or until the
// context is cancelled, when it returns its error
func (r *Registry) waitIdle(ctx context.Context, id uuid.UUID, idle, maxWait time.Duration) error {
	start := time.Now()
	since := start
	if session, err := r.sessionStore.Get(ctx, id); err == nil && len(session.Messages) > 0 {
		if last := session.Messages[len(session.Messages)-1].Time; !last.IsZero() && last.Before(start) {
			since = last
		}
	}
	for {
		now := time.Now()
		if r.sessionStatus(id) != schema.SessionIdle {
			since = now
		} else if now.Sub(since) >= idle {
			return nil
		}
		if now.Sub(start) >= maxWait {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(r.delivery.poll):
		}
	}
}

// appendMessage adds the message, which is an event's, to the session,
// with the time now, unless the session has it already, and stores it, and
// publishes that it changed, and returns whether it was added. Returns
// ErrNotFound when there's no session, and ErrConflict when it changed as it
// was added.
func (r *Registry) appendMessage(ctx context.Context, id uuid.UUID, message providerschema.Message) (bool, error) {
	r.lifecycle.RLock()
	defer r.lifecycle.RUnlock()
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return false, err
	}
	for _, other := range session.Messages {
		if other.Event != nil && message.Event != nil && other.Event.ID == message.Event.ID {
			return false, nil
		}
	}
	message.Time = time.Now().UTC()
	session.Messages = append(session.Messages, message)
	if session, err = r.sessionStore.Put(ctx, session); err != nil {
		return false, err
	}
	r.sessionChanged(eventschema.EventUpdate, session)
	return true, nil
}

// resourcesSince returns the resources of the session which were made since
// the time, apart from those attached to the person's messages, the first
// made first
func (r *Registry) resourcesSince(ctx context.Context, session uuid.UUID, since time.Time, exclude []providerschema.Message) ([]resourceschema.Resource, error) {
	if r.resources == nil || session == uuid.Nil() {
		return nil, nil
	}
	attached := make(map[string]bool)
	for _, message := range exclude {
		if message.Role() != "user" {
			continue
		}
		for _, block := range message.Content {
			if block.Resource != nil && block.Resource.URL != nil {
				if hash, err := resourceschema.ParseURI(block.Resource.URL.String()); err == nil {
					attached[hash] = true
				}
			}
		}
	}
	var made []resourceschema.Resource
	limit := uint(resourceschema.MaxListLimit)
	for offset := uint(0); ; offset += limit {
		page, err := r.resources.List(ctx, resourceschema.ResourceListRequest{Session: session.String(), Offset: offset, Limit: &limit})
		if err != nil {
			return nil, err
		}
		for _, resource := range page.Body {
			if !resource.Created.Before(since) && !attached[resource.Hash] {
				made = append(made, resource)
			}
		}
		if uint(len(page.Body)) < limit {
			break
		}
	}
	slices.SortFunc(made, func(a, b resourceschema.Resource) int {
		if c := a.Created.Compare(b.Created); c != 0 {
			return c
		}
		return strings.Compare(a.Hash, b.Hash)
	})
	return made, nil
}

// copyRunResources copies the resources of the event's run, which it made
// since it started, to the session the event is for, and returns them, as
// the session's, the first made first. They're the run's session's, rather
// than those its messages refer to, so those agents it called made, and
// those of results which were truncated, are included.
func (r *Registry) copyRunResources(ctx context.Context, event schema.Event, since time.Time) ([]resourceschema.Resource, error) {
	if r.resources == nil || event.Run == uuid.Nil() {
		return nil, nil
	}
	made, err := r.resourcesSince(ctx, event.Run, since, nil)
	if err != nil {
		return nil, err
	}
	copied := make([]resourceschema.Resource, 0, len(made))
	for _, resource := range made {
		resource, err := r.resources.Copy(ctx, resource.URI(), event.Run, event.Session)
		if err != nil {
			return nil, err
		}
		copied = append(copied, resource)
	}
	return copied, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// runStarted returns when the event's run started, at the latest, which is
// when it fired, or was due, or its outcome was created
func runStarted(event schema.Event) time.Time {
	switch {
	case !event.Fired.IsZero():
		return event.Fired
	case !event.Scheduled.IsZero():
		return event.Scheduled
	}
	return event.Created
}

// resourceLinks returns the resources, as links to them
func resourceLinks(resources []resourceschema.Resource) []toolschema.ResourceLink {
	links := make([]toolschema.ResourceLink, 0, len(resources))
	for _, resource := range resources {
		links = append(links, toolschema.ResourceLink{URI: resource.URI(), Name: resource.Name, Title: resource.Description, Type: resource.Type, Size: resource.Size})
	}
	return links
}
