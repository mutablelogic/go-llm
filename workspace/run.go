package workspace

import (
	"context"
	"encoding/json"
	"log/slog"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run logs the registry's changes, with the logger, as they're published to
// its broadcaster: the workspaces which are added, or removed, and their
// folders, and resources, which change, until the context is cancelled, when
// it returns nil. Without a broadcaster, it waits until then. Returns
// ErrConflict when it's running already.
func (r *Registry) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if !r.running.CompareAndSwap(false, true) {
		return llm.ErrConflict.With("workspaces are running already")
	}
	defer r.running.Store(false)

	// Without a broadcaster, there's nothing to log
	if r.broadcaster == nil {
		<-ctx.Done()
		return nil
	}

	// Log the registry's events, until it's cancelled, or the broadcaster
	// stops
	ch := make(chan eventschema.Event, 16)
	done, err := r.broadcaster.Watch(ctx, ch)
	if err != nil {
		return err
	}
	for {
		select {
		case event := <-ch:
			if event.Schema == EventSchema {
				r.log(ctx, logger, event)
			}
		case err := <-done:
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}

// Running returns true while the registry runs, once Run, which is started
// in a goroutine, has started, and until it stops
func (r *Registry) Running() bool {
	return r.running.Load()
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// log logs the registry's event: a workspace which is added, or removed, or
// a folder, or resource, of one, which changes
func (r *Registry) log(ctx context.Context, logger *slog.Logger, event eventschema.Event) {
	switch event.Table {
	case WorkspaceTable:
		switch event.Action {
		case eventschema.EventCreate:
			logger.InfoContext(ctx, "workspace added", "workspace", event.Row)
		case eventschema.EventDelete:
			logger.InfoContext(ctx, "workspace removed", "workspace", event.Row)
		}
	case ResourceTable:
		var data ResourceEvent
		_ = json.Unmarshal(event.Data, &data)
		logger.DebugContext(ctx, "workspace changed", "workspace", event.Row, "action", string(event.Action), "uri", data.URI)
	}
}
