package registry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run keeps the registry current, until the context is cancelled, when it
// returns nil: it pings the providers, and lists the models of those whose
// models are stale, when it starts, and every PingInterval, so the
// providers' state, and their models, are kept current, and with a store,
// it syncs the registry's providers with the store's, when it starts, as
// either changes, and every PingInterval, as an event can be missed. The
// errors syncing them, which are synced again later, are logged with the
// logger, or aren't when it's nil. When the store stops, it returns its
// error. The registry is used without it, when its models are listed as
// they're needed. When it stops, the registry's watches end, as there will
// be no more events, and it isn't watched until it runs again.
func (r *Registry) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	r.startWatching()
	defer r.stopWatching()

	// The store's watches end when it returns, as well as when the context
	// is cancelled, such as when the store stops, so the registry isn't left
	// sending its events to a channel which isn't read
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// With a store, watch the registry, and the store, before they're
	// synced, so no change is missed, and sync them. Without one, the
	// channels are nil, so they're never received from.
	var s *syncer
	var registryEvents, storeEvents chan providerschema.Event
	var storeDone <-chan error
	if r.store != nil {
		s = &syncer{registry: r, store: r.store, known: make(map[string]schema.Provider)}
		registryEvents = make(chan providerschema.Event, syncEventBuffer)
		if _, err := r.Watch(ctx, registryEvents); err != nil {
			return err
		}
		storeEvents = make(chan providerschema.Event, syncEventBuffer)
		done, err := r.store.Watch(ctx, storeEvents)
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

	// Sync, and refresh, and then as they change, and every PingInterval
	if s != nil {
		logError(s.sync(ctx, registryEvents))
	}
	r.refresh(ctx)
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-storeDone:
			return err
		case event := <-registryEvents:
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
// can be reached, and whose models are stale, within half of PingInterval
func (r *Registry) refresh(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, PingInterval/2)
	defer cancel()
	each(r.entries(), func(_ int, e entry) {
		if err := e.cache.ping(ctx, 0); err == nil && e.cache.stale() {
			e.cache.refresh(ctx)
		}
	})
}
