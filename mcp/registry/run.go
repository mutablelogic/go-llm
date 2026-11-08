package registry

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	client "github.com/mutablelogic/go-llm/mcp/client"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// RetryInterval is how often Run connects the connectors which aren't
	// connected, such as those whose servers couldn't be, or closed their
	// sessions, and connects, and disconnects, those which another process
	// with the store created, or deleted. A connector which the registry
	// creates, updates, or deletes is connected, or disconnected, at once.
	RetryInterval = 30 * time.Second

	// ConnectTimeout is how long a connector's server is waited for, as it's
	// connected, and its tools are listed, unless the registry is given
	// another, with WithConnectTimeout
	ConnectTimeout = 30 * time.Second

	// ConnectJitter is the most each connector waits, at random, before
	// it's connected, when more than one is, such as as Run starts, so
	// they aren't all connected at once
	ConnectJitter = 2 * time.Second
)

const (
	// eventBuffer is how many of the broadcaster's events are kept, as the
	// connectors are reconciled, which covers every change before it, so one
	// which is missed, as the channel is full, doesn't matter
	eventBuffer = 16
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run connects the connectors' servers, as it starts, and disconnects
// those which are deleted, at once, as the registry's own broadcaster
// publishes that they're created, updated, or deleted, and every RetryInterval, for
// those which aren't connected, until the context is cancelled, when it
// disconnects every one, and returns. What happens is logged, with the
// logger, or isn't when it's nil.
func (r *Registry) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	r.logger.Store(logger)
	defer func() {
		if err := r.Close(); err != nil {
			logger.WarnContext(ctx, "connectors not disconnected", "error", err.Error())
		}
	}()

	// Watch the broadcaster, before the connectors are first reconciled, so
	// no change is missed
	events := make(chan eventschema.Event, eventBuffer)
	done, err := r.events.Watch(ctx, events)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(RetryInterval)
	defer ticker.Stop()

	// Reconcile them, as they change, and every interval
	for reconcile := true; ; {
		if reconcile {
			r.reconcile(ctx, logger)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			// The broadcaster stopped, which the registry's own doesn't, but
			// they're reconciled every interval, if it does
			events, done = nil, nil
			reconcile = false
		case event := <-events:
			reconcile = event.Schema == schema.Name && event.Table == schema.ConnectorTable
		case <-ticker.C:
			reconcile = true
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// reconcile connects the connectors which aren't connected, at once, and
// disconnects the clients of those which the store hasn't, any more
func (r *Registry) reconcile(ctx context.Context, logger *slog.Logger) {
	connectors, err := r.store.List(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logger.ErrorContext(ctx, "connectors not listed", "error", err.Error())
		}
		return
	}

	// Each has a namespace, connected, or not, and the clients of those
	// which were deleted are disconnected, and their namespaces removed
	names := make(map[string]bool, len(connectors))
	for _, connector := range connectors {
		names[connector.Name] = true
		r.ensureNamespace(connector)
	}
	r.mu.Lock()
	var deleted []string
	for name := range r.clients {
		if !names[name] {
			deleted = append(deleted, name)
		}
	}
	for name := range r.namespaces {
		if _, exists := r.clients[name]; !exists && !names[name] {
			deleted = append(deleted, name)
		}
	}
	r.mu.Unlock()
	for _, name := range deleted {
		r.forget(name, 0)
		logger.InfoContext(ctx, "connector disconnected", "connector", name, "reason", "deleted")
	}

	// The connectors which aren't connected
	type pending struct {
		connector schema.Connector
		client    *client.Client
	}
	var connect []pending
	for _, connector := range connectors {
		c, err := r.client(connector)
		switch {
		case errors.Is(err, llm.ErrConflict), errors.Is(err, llm.ErrNotFound):
			// It was changed, or deleted, since it was read
		case err != nil:
			logger.ErrorContext(ctx, "connector has no client", "connector", connector.Name, "error", err.Error())
		case !c.Connected():
			connect = append(connect, pending{connector, c})
		default:
			// Its tools are listed again, when they couldn't be, as it was
			// connected
			r.mu.Lock()
			ns := r.namespaces[connector.Name]
			r.mu.Unlock()
			if server, _ := c.Status(); ns != nil && server != nil && !ns.ConnectedTo(c) {
				r.connectNamespace(c, server, connector)
			}
		}
	}

	// Connect them at once, each after a while, at random, when there's
	// more than one, so they're spread over ConnectJitter
	var wg sync.WaitGroup
	for _, p := range connect {
		var delay time.Duration
		if len(connect) > 1 && ConnectJitter > 0 {
			delay = rand.N(ConnectJitter)
		}
		wg.Go(func() {
			if delay > 0 {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
			}
			r.connect(ctx, logger, p.connector, p.client)
		})
	}
	wg.Wait()
}

// connect connects the client of the connector, within its connect timeout, and
// logs that it was, or why it wasn't, when that's changed
func (r *Registry) connect(ctx context.Context, logger *slog.Logger, connector schema.Connector, c *client.Client) {
	_, before := c.Status()
	connectCtx, cancel := context.WithTimeout(ctx, r.connectTimeout)
	defer cancel()
	if err := c.Connect(connectCtx); err != nil {
		if ctx.Err() == nil && (before == nil || before.Error() != err.Error()) {
			logger.WarnContext(ctx, "connector not connected", "connector", connector.Name, "url", connector.URL, "error", err.Error())
		}
		return
	} else if !r.current(connector.Name, c) {
		// It was replaced, or forgotten, as it was connected, such as when
		// the connector was updated, so it isn't left connected
		_ = c.Disconnect()
		return
	}
	server, _ := c.Status()
	attrs := []any{"connector", connector.Name, "url", connector.URL}
	if server != nil {
		attrs = append(attrs, "server", server.Name, "version", server.Version)
	}
	logger.InfoContext(ctx, "connector connected", attrs...)
}
