package registry

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	// Packages
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

// DeleteOldSessions deletes the sessions which haven't changed for longer
// than they're kept, as Run does, and returns how many it deleted
func (r *Registry) DeleteOldSessions(ctx context.Context) int {
	return r.deleteOldSessions(ctx, slog.New(slog.DiscardHandler))
}

// DeleteSessionAt deletes the session, when the serial is its, as the
// sessions which haven't changed for longer than they're kept are
func (r *Registry) DeleteSessionAt(ctx context.Context, id uuid.UUID, serial uint64) error {
	_, err := r.deleteSession(ctx, id, &serial)
	return err
}

// Deliver delivers the event's outcome, with the resources of its run's
// messages, as RunEvent does, in the background, and returns its error
func (r *Registry) Deliver(ctx context.Context, event schema.Event, run []providerschema.Message) error {
	return r.deliver(ctx, event, run)
}

// Replying sets the session replying, until the function it returns is
// called
func (r *Registry) Replying(id uuid.UUID) func() {
	return r.replying(id)
}

// SetDelivery sets how long a session must be idle before an event's
// outcome is delivered, how long a notification, and an alert, wait for it
// at most, and how often it's looked at, before the registry delivers any
func (r *Registry) SetDelivery(idle, maxWait, alertWait, poll time.Duration) {
	r.delivery = delivery{idle: idle, maxWait: maxWait, alertWait: alertWait, poll: poll, retries: defaultDelivery.retries}
}
