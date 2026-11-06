package heartbeat

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// eventBuffer is how many events are kept, until they're read, by a
	// goroutine which does nothing else, so they're read as they're sent
	eventBuffer = 256

	// tickDelay is how long after the start of each minute the heartbeats
	// which are due fire, so a time is never just before it
	tickDelay = 100 * time.Millisecond

	// cleanupInterval is how often the heartbeats which won't fire again are
	// looked for, to delete those kept for longer than they're kept
	cleanupInterval = 24 * time.Hour
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run fires the heartbeats which are due, when it starts, and at the start
// of each minute, until the context is cancelled, when it returns nil. A
// heartbeat which was due more than once, such as when the server wasn't
// running, fires once, when it was first due. With a broadcaster, it deletes
// the heartbeats for each session which is deleted, and, with WithSessions,
// a heartbeat whose session doesn't exist when it's due, as the event may
// have been missed, is deleted, rather than fired. A heartbeat which won't
// fire again, such as a one-off which fired, is deleted once it's been kept
// for a week, or the time WithRetention sets, after it last fired, or was
// changed, which is looked for when it starts, and daily. What fails, such
// as a store which can't be read, is logged to the logger, when there is one.
func (h *Heartbeats) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	// The deleted sessions, whose heartbeats are deleted, which a goroutine
	// reads as they're sent, so none is missed while what's due fires, and
	// queues, until they're deleted
	var deleted sessionQueue
	if h.broadcaster != nil {
		events := make(chan eventschema.Event, eventBuffer)
		if done, err := h.broadcaster.Watch(ctx, events); err != nil {
			logger.WarnContext(ctx, "heartbeats of deleted sessions won't be deleted, until they're due", "error", err)
		} else {
			go deleted.read(events, done)
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
		case <-deleted.ready():
			for _, session := range deleted.take() {
				h.deleteSession(ctx, logger, session)
			}
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

	// Those which won't fire again, once they've been kept long enough, which
	// are looked for daily
	if h.cleaned.IsZero() || now.Sub(h.cleaned) >= cleanupInterval {
		h.cleaned = now
		heartbeats = h.cleanup(ctx, logger, heartbeats, now)
	}
	for _, heartbeat := range heartbeats {
		if next := h.next(heartbeat); next != nil && !next.After(now) {
			if h.session != nil {
				if err := h.session(ctx, heartbeat.Session); errors.Is(err, llm.ErrNotFound) {
					h.deleteSession(ctx, logger, heartbeat.Session)
					continue
				}
			}
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

// cleanup deletes the heartbeats which won't fire again, which were last
// fired, or changed, longer ago than they're kept, and returns the others
func (h *Heartbeats) cleanup(ctx context.Context, logger *slog.Logger, heartbeats []schema.Heartbeat, now time.Time) []schema.Heartbeat {
	kept := heartbeats[:0]
	deleted := 0
	for _, heartbeat := range heartbeats {
		since := heartbeat.Modified
		if heartbeat.Fired != nil && heartbeat.Fired.After(since) {
			since = *heartbeat.Fired
		}
		if h.next(heartbeat) != nil || now.Sub(since) < h.retention {
			kept = append(kept, heartbeat)
			continue
		}
		if _, err := h.Delete(ctx, heartbeat.ID); err != nil && !errors.Is(err, llm.ErrNotFound) {
			logger.ErrorContext(ctx, "heartbeat not deleted", "heartbeat", heartbeat.ID, "title", heartbeat.Title, "error", err)
			kept = append(kept, heartbeat)
			continue
		}
		deleted++
	}
	if deleted > 0 {
		logger.InfoContext(ctx, "heartbeats which won't fire again deleted", "heartbeats", deleted)
	}
	return kept
}

// deleteSession deletes the heartbeats for the session, which was deleted
func (h *Heartbeats) deleteSession(ctx context.Context, logger *slog.Logger, session uuid.UUID) {
	if n, err := h.DeleteSession(ctx, session); err != nil {
		logger.ErrorContext(ctx, "heartbeats of deleted session not deleted", "session", session, "error", err)
	} else if n > 0 {
		logger.InfoContext(ctx, "heartbeats of deleted session deleted", "session", session, "heartbeats", n)
	}
}

///////////////////////////////////////////////////////////////////////////////
// SESSION QUEUE

// sessionQueue is the sessions which were deleted, in the order they were,
// until they're taken, and a channel which is ready when there are any
type sessionQueue struct {
	mu       sync.Mutex
	sessions []uuid.UUID
	notify   chan struct{}
}

// read queues the sessions of the events which are deleted, until the watch
// ends
func (q *sessionQueue) read(events <-chan eventschema.Event, done <-chan error) {
	for {
		select {
		case <-done:
			return
		case event := <-events:
			if event.Schema != registry.EventSchema || event.Table != registry.SessionTable || event.Action != eventschema.EventDelete {
				continue
			}
			if session, err := uuid.Parse(event.Row); err == nil {
				q.mu.Lock()
				q.sessions = append(q.sessions, session)
				q.mu.Unlock()
				select {
				case q.channel() <- struct{}{}:
				default:
				}
			}
		}
	}
}

// ready returns the channel which is ready when sessions are queued
func (q *sessionQueue) ready() <-chan struct{} {
	return q.channel()
}

// take returns the sessions which are queued, which are no longer queued
func (q *sessionQueue) take() []uuid.UUID {
	q.mu.Lock()
	defer q.mu.Unlock()
	sessions := q.sessions
	q.sessions = nil
	return sessions
}

// channel returns the channel which is notified, which is made once
func (q *sessionQueue) channel() chan struct{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.notify == nil {
		q.notify = make(chan struct{}, 1)
	}
	return q.notify
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// untilMinute returns how long it is until just after the start of the next
// minute
func untilMinute(now time.Time) time.Duration {
	return now.Truncate(time.Minute).Add(time.Minute + tickDelay).Sub(now)
}
