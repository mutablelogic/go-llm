package registry

import (
	"context"
	"log/slog"
	"strconv"

	// Packages
	store "github.com/mutablelogic/go-llm/webhook/store"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// eventBuffer is how many changes are kept, until they're read, so a burst
// of calls isn't missed
const eventBuffer = 256

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run watches the store's changes, with the broadcaster it publishes them
// to, and logs them to the logger: the webhooks which are created, changed
// and deleted, and the calls which are received, which were accepted, or
// refused, with why, and deleted, until the context is cancelled, when it
// returns nil. Without a broadcaster, it waits until then. It returns the
// broadcaster's error, when it stops running first.
func (r *Registry) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if r.broadcaster == nil {
		<-ctx.Done()
		return nil
	}

	// Watch the store's changes, of its schema
	changes := make(chan eventschema.Change, eventBuffer)
	done, err := r.broadcaster.Watch(ctx, eventschema.ChangeFilter{Schema: store.Schema}, changes)
	if err != nil {
		return stopped(ctx, err)
	}
	for {
		select {
		case err := <-done:
			return stopped(ctx, err)
		case change := <-changes:
			r.log(ctx, logger, change)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// log logs the change: a call, which was received, with what it was, which
// is read, as the change only has its serial number, or deleted, or a
// webhook, which was created, changed, or deleted, or that changes were
// missed, or were too many, which is a change without a row
func (r *Registry) log(ctx context.Context, logger *slog.Logger, change eventschema.Change) {
	if change.Row == "" {
		logger.DebugContext(ctx, "webhook changes missed", "table", change.Table)
		return
	}
	switch {
	case change.Table == store.CallTable && change.Action == eventschema.ChangeCreate:
		id, err := strconv.ParseUint(change.Row, 10, 64)
		if err != nil {
			return
		}
		call, err := r.store.GetCall(ctx, id)
		if err != nil {
			logger.DebugContext(ctx, "webhook call not read", "call", id, "error", err.Error())
			return
		}
		attrs := []any{"call", call.ID, "webhook", call.Webhook}
		if call.Event != "" {
			attrs = append(attrs, "event", call.Event)
		}
		if call.Delivery != "" {
			attrs = append(attrs, "delivery", call.Delivery)
		}
		if call.Error != "" {
			logger.WarnContext(ctx, "webhook call refused", append(attrs, "error", call.Error)...)
		} else {
			logger.InfoContext(ctx, "webhook call accepted", attrs...)
		}
	case change.Table == store.CallTable && change.Action == eventschema.ChangeDelete:
		logger.DebugContext(ctx, "webhook call deleted", "call", change.Row)
	case change.Table == store.WebhookTable:
		logger.InfoContext(ctx, "webhook "+actionName(change.Action), "webhook", change.Row)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// stopped returns the error of a watch which ended, or nil, when the context
// is cancelled, as the broadcaster can stop running at the same time, when
// the server stops, whose error the watch can end with
func stopped(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// actionName returns what happened to a webhook, such as created
func actionName(action eventschema.ChangeAction) string {
	switch action {
	case eventschema.ChangeCreate:
		return "created"
	case eventschema.ChangeUpdate:
		return "changed"
	case eventschema.ChangeDelete:
		return "deleted"
	}
	return string(action)
}
