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

// ShortenedResultsPrompt is what a model with tools is told about results
// which are shortened to fit its context
const ShortenedResultsPrompt = shortenedResultsPrompt

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

// Deliver delivers the event's outcome, with the resources its run made
// since the time, as RunEvent does, in the background, and returns its error
func (r *Registry) Deliver(ctx context.Context, event schema.Event, since time.Time) error {
	return r.deliver(ctx, event, since)
}

// RetryDeliveries delivers the outcomes which are pending, which weren't,
// as Run does, every so often
func (r *Registry) RetryDeliveries(ctx context.Context) {
	r.retryDeliveries(ctx)
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
	r.delivery = defaultDelivery
	r.delivery.idle, r.delivery.maxWait, r.delivery.alertWait, r.delivery.poll = idle, maxWait, alertWait, poll
}

// Summarizing marks the marker's summary as being written, as it is while
// it's written in the background, until the function it returns is called
func (r *Registry) Summarizing(marker uuid.UUID) func() {
	r.summarizing.Store(marker, struct{}{})
	return func() { r.summarizing.Delete(marker) }
}

// RecountContext sets the tokens of the session's conversation to those of
// its context
func RecountContext(session *schema.Session, before string) {
	recountContext(session, before)
}

// SummaryTokens returns about how many tokens a summary is, in the system
// prompt
func SummaryTokens(summary string) uint {
	return summaryTokens(summary)
}

// ContextSummary returns the summary of the messages before a conversation's
// context
func ContextSummary(messages []providerschema.Message) string {
	_, summary := contextOf(messages)
	return summary
}

// EventOrigin returns what the system says of an event's message
func EventOrigin(event schema.Event) string {
	return eventOrigin(event)
}

// ProviderErr returns the provider's error, as an llm error, in words
func ProviderErr(err error) error {
	return providerErr(err)
}
