package heartbeat

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// eventBuffer is how many events of sessions are kept, until they're
	// read
	eventBuffer = 64

	// tickDelay is how long after the start of each minute the heartbeats
	// which are due fire, so a time is never just before it
	tickDelay = 100 * time.Millisecond
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run fires the heartbeats which are due, when it starts, and at the start
// of each minute, until the context is cancelled, when it returns nil. A
// heartbeat which was due more than once, such as when the server wasn't
// running, fires once, when it was first due. With a broadcaster, it deletes
// the heartbeats for each session which is deleted. What fails, such as a
// store which can't be read, is logged to the logger, when there is one.
func (h *Heartbeats) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	// The deleted sessions, whose heartbeats are deleted
	var events chan eventschema.Event
	var done <-chan error
	if h.broadcaster != nil {
		events = make(chan eventschema.Event, eventBuffer)
		var err error
		if done, err = h.broadcaster.Watch(ctx, events); err != nil {
			logger.WarnContext(ctx, "heartbeats of deleted sessions won't be deleted", "error", err)
			events, done = nil, nil
		}
	}

	// Fire what's due, and then again at the start of each minute
	h.tick(ctx, logger)
	timer := time.NewTimer(untilMinute(h.now()))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			events, done = nil, nil
		case event := <-events:
			h.sessionEvent(ctx, logger, event)
		case <-timer.C:
			h.tick(ctx, logger)
			timer.Reset(untilMinute(h.now()))
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// tick fires the heartbeats which are due
func (h *Heartbeats) tick(ctx context.Context, logger *slog.Logger) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	now := h.now()
	heartbeats, err := h.store.List(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "heartbeats not read", "error", err)
		return
	}
	for _, heartbeat := range heartbeats {
		if next := h.next(heartbeat); next != nil && !next.After(now) {
			if err := h.fire(ctx, heartbeat.ID, *next, now); err != nil {
				logger.ErrorContext(ctx, "heartbeat not fired", "heartbeat", heartbeat.ID, "title", heartbeat.Title, "error", err)
			}
		}
	}
}

// fire fires the heartbeat with the identifier, which was due at the time,
// when it's still due then, as it is in the store, so only one server fires
// it, and publishes that it fired
func (h *Heartbeats) fire(ctx context.Context, id uuid.UUID, scheduled, now time.Time) (err error) {
	ctx, endSpan := otel.StartSpan(h.tracer, ctx, "heartbeat.Fire",
		attribute.String("arg.id", id.String()),
		attribute.String("arg.scheduled", scheduled.Format(time.RFC3339)),
	)
	defer func() { endSpan(err) }()

	heartbeat, err := h.store.Update(ctx, id, func(heartbeat *schema.Heartbeat) error {
		if next := h.next(*heartbeat); next == nil || !next.Equal(scheduled) {
			return errNotDue
		}
		heartbeat.Fired = &now
		return nil
	})
	switch {
	case errors.Is(err, errNotDue):
		return nil
	case err != nil:
		return err
	}
	h.publishFire(schema.FireEvent{
		Heartbeat: heartbeat.ID,
		Title:     heartbeat.Title,
		Prompt:    heartbeat.Prompt,
		Session:   heartbeat.Session,
		Scheduled: scheduled.UTC(),
		Fired:     now.UTC(),
	})
	h.publish(eventschema.EventUpdate, heartbeat)
	return nil
}

// sessionEvent deletes the heartbeats for the session of the event, when
// it's deleted
func (h *Heartbeats) sessionEvent(ctx context.Context, logger *slog.Logger, event eventschema.Event) {
	if event.Schema != registry.EventSchema || event.Table != registry.SessionTable || event.Action != eventschema.EventDelete {
		return
	}
	session, err := uuid.Parse(event.Row)
	if err != nil {
		return
	}
	if n, err := h.DeleteSession(ctx, session); err != nil {
		logger.ErrorContext(ctx, "heartbeats of deleted session not deleted", "session", session, "error", err)
	} else if n > 0 {
		logger.InfoContext(ctx, "heartbeats of deleted session deleted", "session", session, "heartbeats", n)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// untilMinute returns how long it is until just after the start of the next
// minute
func untilMinute(now time.Time) time.Duration {
	return now.Truncate(time.Minute).Add(time.Minute + tickDelay).Sub(now)
}
