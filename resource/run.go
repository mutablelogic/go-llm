package resource

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/resource/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// eventBuffer is how many events of sessions which are deleted are
	// buffered, before they're read
	eventBuffer = 100

	// errNotExpired is returned by a delete which cleans up a resource,
	// which was used since it was listed, so it isn't deleted
	errNotExpired = errString("resource isn't expired")

	// sessionSchema and sessionTable are the schema and table of the
	// registry's events of sessions, which are registry.EventSchema and
	// registry.SessionTable, which aren't imported, as the registry reads
	// resources
	sessionSchema = "llm"
	sessionTable  = "session"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run deletes the resources of each session which is deleted, with a
// broadcaster, which unlinks their content, and deletes what isn't needed, when it starts, and every
// schema.CleanInterval: resources on their own which weren't used for the
// retention, those whose session doesn't exist, with WithSessions, as its
// event may have been missed, and content which no resource refers to,
// until the context is cancelled, when it returns nil. What fails is logged
// to the logger, when there is one.
func (r *Manager) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	// The deleted sessions, whose resources are deleted, which a goroutine
	// reads as they're sent, and queues, until they're deleted
	var deleted sessionQueue
	if r.broadcaster != nil {
		events := make(chan eventschema.Event, eventBuffer)
		if done, err := r.broadcaster.Watch(ctx, events); err != nil {
			logger.WarnContext(ctx, "resources of deleted sessions won't be deleted, until they're cleaned", "error", err.Error())
		} else {
			go deleted.read(events, done)
		}
	}

	// Clean when it starts, and then every interval
	r.Clean(ctx, logger)
	ticker := time.NewTicker(schema.CleanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-deleted.ready():
			for _, session := range deleted.take() {
				if n, err := r.DeleteSession(ctx, session); err != nil {
					logger.ErrorContext(ctx, "resources of deleted session not deleted", "session", session, "error", err.Error())
				} else if n > 0 {
					logger.InfoContext(ctx, "resources of deleted session deleted", "session", session, "resources", n)
				}
			}
		case <-ticker.C:
			r.Clean(ctx, logger)
		}
	}
}

// Clean deletes the resources on their own which weren't used for the
// retention, those whose session doesn't exist, with WithSessions, and then
// content which no resource refers to. What fails is logged.
func (r *Manager) Clean(ctx context.Context, logger *slog.Logger) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	all, err := r.store.List(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "resources not read", "error", err.Error())
		return
	}

	// The resources which aren't needed
	now := r.now()
	deleted := 0
	for _, resource := range all {
		if !r.expired(ctx, resource, now) {
			continue
		}
		// It's deleted only when it still isn't needed, as it may have been
		// used since it was listed
		_, err := r.store.Delete(ctx, resource.Key(), func(current schema.Resource) error {
			if !r.expired(ctx, current, now) {
				return errNotExpired
			}
			return nil
		})
		if errors.Is(err, errNotExpired) || errors.Is(err, llm.ErrNotFound) {
			continue
		} else if err != nil {
			logger.ErrorContext(ctx, "resource not deleted", "resource", resource.Key().String(), "error", err.Error())
		} else {
			r.publish(eventschema.EventDelete, resource)
			deleted++
		}
	}
	if deleted > 0 {
		logger.InfoContext(ctx, "resources which weren't needed deleted", "resources", deleted)
	}

	// The content which no resource refers to, with the lock held, so none
	// is written as it's removed
	r.contents.Lock()
	defer r.contents.Unlock()
	removed, err := r.removeUnreferenced(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "resource content not removed", "error", err.Error())
	} else if removed > 0 {
		logger.InfoContext(ctx, "resource content which wasn't needed removed", "contents", removed)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// expired returns whether the resource isn't needed: it's on its own, and
// wasn't used for the retention, or its session doesn't exist
func (r *Manager) expired(ctx context.Context, resource schema.Resource, now time.Time) bool {
	if resource.Session == nil {
		return now.Sub(resource.Used) >= r.retention
	} else if r.session == nil {
		return false
	}
	return errors.Is(r.session(ctx, *resource.Session), llm.ErrNotFound)
}

// removeUnreferenced removes the content which no resource refers to, and
// returns how much it removed, which is called with the lock held
func (r *Manager) removeUnreferenced(ctx context.Context) (int, error) {
	contents, err := r.store.Contents(ctx)
	if err != nil {
		return 0, err
	}
	all, err := r.store.List(ctx)
	if err != nil {
		return 0, err
	}
	referenced := make(map[string]bool, len(all))
	for _, resource := range all {
		referenced[resource.Hash] = true
	}
	var errs []error
	removed := 0
	for _, hash := range contents {
		if referenced[hash] {
			continue
		}
		if err := r.store.Remove(ctx, hash); err != nil {
			errs = append(errs, err)
		} else {
			removed++
		}
	}
	return removed, errors.Join(errs...)
}

// errString is an error, as a constant
type errString string

func (e errString) Error() string {
	return string(e)
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
			if event.Schema != sessionSchema || event.Table != sessionTable || event.Action != eventschema.EventDelete {
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
