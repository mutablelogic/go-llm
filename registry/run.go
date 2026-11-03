package registry

import (
	"context"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run pings the providers, and lists the models of those whose models are
// stale, when it starts, and every PingInterval, so the providers' state,
// and their models, are kept current, until the context is cancelled. The
// registry is used without it, when its models are listed as they're
// needed.
func (r *Registry) Run(ctx context.Context) error {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()
	for {
		r.refresh(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
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
