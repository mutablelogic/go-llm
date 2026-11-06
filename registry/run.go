package registry

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ready returns a channel which is closed once Run has loaded the stored
// agents, and synced the providers, and their models, when it starts, so
// what needs them, such as heartbeats which are overdue, waits for it
func (r *Registry) Ready() <-chan struct{} {
	return r.ready
}

// Run keeps the registry current, until the context is cancelled, when it
// returns nil: it pings the providers, and lists the models of those whose
// models are stale, when it starts, and every PingInterval, so the
// providers' state, and their models, are kept current, and with a store,
// it syncs the registry's providers with the store's, when it starts, as
// either changes, and every PingInterval, as an event can be missed. The
// errors syncing them, which are synced again later, are logged with the
// logger, or aren't when it's nil. When the store stops, it returns its
// error. The registry is used without it, when its models are listed as
// they're needed. With a store of sessions, the changes others make to
// them, which it sends as events, are published as the registry's own. With
// a store of agents, and a toolkit, the stored agents are made tools, when
// it starts.
func (r *Registry) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	// The stored agents are tools, which a store which can't be read leaves
	// without them, until they're changed
	if err := r.loadAgents(ctx); err != nil {
		logger.ErrorContext(ctx, "loading agents", "error", err)
	}
	// The store's watches end when it returns, as well as when the context
	// is cancelled, such as when the store stops, so the registry isn't left
	// sending its events to a channel which isn't read
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// With a store, watch the registry, and the store, before they're
	// synced, so no change is missed, and sync them. Without one, the
	// channels are nil, so they're never received from.
	var s *syncer
	var registryEvents, storeEvents chan eventschema.Event
	var storeDone <-chan error
	if r.providerStore != nil {
		s = &syncer{registry: r, store: r.providerStore, known: make(map[string]schema.Provider)}
		registryEvents = make(chan eventschema.Event, syncEventBuffer)
		if _, err := r.Watch(ctx, registryEvents); err != nil {
			return err
		}
		storeEvents = make(chan eventschema.Event, syncEventBuffer)
		done, err := r.providerStore.Watch(ctx, storeEvents)
		if err != nil {
			return err
		}
		storeDone = done
	}

	// logError logs an error syncing the providers, which are synced again
	// later
	logError := func(err error) {
		if err != nil {
			logger.ErrorContext(ctx, "syncing providers", "error", err)
		}
	}

	// With a store of sessions, publish the changes others make to them
	var sessionEvents chan eventschema.Event
	var sessionDone <-chan error
	if r.sessionStore != nil {
		sessionEvents = make(chan eventschema.Event, syncEventBuffer)
		done, err := r.sessionStore.Watch(ctx, sessionEvents)
		if err != nil {
			return err
		}
		sessionDone = done
	}

	// Sync, and refresh, and then as they change, and every PingInterval
	if s != nil {
		logError(s.sync(ctx, registryEvents))
	}
	r.refresh(ctx)
	r.readyOnce.Do(func() { close(r.ready) })
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-storeDone:
			return err
		case err := <-sessionDone:
			return err
		case event := <-sessionEvents:
			// The store's events are the registry's, whatever schema the
			// store says they are of
			event.Schema = EventSchema
			// A session which was deleted elsewhere has no status here
			// either, so it's not published as idle after it. As with
			// DeleteSession, the lifecycle lock is held, so a status isn't
			// set as, or after, it's deleted.
			if event.Action == eventschema.EventDelete {
				r.lifecycle.Lock()
				if id, err := uuid.Parse(event.Row); err == nil {
					r.forgetStatus(id)
				}
				r.broadcaster.Publish(event)
				r.lifecycle.Unlock()
			} else {
				r.broadcaster.Publish(event)
			}
		case event := <-registryEvents:
			if event.Schema != EventSchema || event.Table != ProviderTable {
				// The broadcaster's other events, such as of sessions
				continue
			}
			if err := s.local(ctx, event.Row); errors.Is(err, llm.ErrConflict) {
				// The store's provider was changed since it was seen, so the
				// providers are synced, and the store's change is kept
				logError(s.sync(ctx, registryEvents))
			} else {
				logError(err)
			}
		case <-storeEvents:
			logError(s.sync(ctx, registryEvents))
		case <-ticker.C:
			if s != nil {
				logError(s.sync(ctx, registryEvents))
			}
			r.refresh(ctx)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// refresh pings each provider, at once, and lists the models of those which
// can be reached, and whose models are stale, within half of PingInterval. A
// provider which was pinged within half of PingInterval, such as when it was
// created, isn't pinged again, as its ping is recent, but one which was
// pinged when the registry last refreshed, a PingInterval ago, is.
func (r *Registry) refresh(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, PingInterval/2)
	defer cancel()
	each(r.entries(), func(_ int, e entry) {
		if err := e.cache.ping(ctx, PingInterval/2); err == nil && e.cache.stale() {
			e.cache.refresh(ctx)
		}
	})
}
