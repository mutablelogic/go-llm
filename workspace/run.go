package workspace

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run runs the workspaces which are Runners, as they're added, until
// they're removed, or the context is cancelled, when it waits for them to
// stop, and returns nil. A workspace which fails is logged, with the logger,
// rather than stopping the others. Returns ErrConflict when it's running
// already.
func (r *Registry) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	r.mu.Lock()
	if r.running != nil {
		r.mu.Unlock()
		return llm.ErrConflict.With("workspaces are running already")
	}
	r.running, r.logger = ctx, logger
	for _, name := range slices.Sorted(maps.Keys(r.workspaces)) {
		r.log("workspace added", name, r.workspaces[name])
		r.start(name, r.workspaces[name])
	}
	r.mu.Unlock()

	// Stop them, once it's cancelled
	<-ctx.Done()
	r.mu.Lock()
	for name, cancel := range r.runs {
		cancel()
		delete(r.runs, name)
	}
	r.running, r.logger = nil, nil
	r.mu.Unlock()
	r.wg.Wait()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// start runs the workspace, when it's a Runner, and the registry is
// running, until its run is cancelled, and logs it, when it fails. The
// caller holds the lock.
func (r *Registry) start(name string, ws Workspace) {
	runner, ok := ws.(Runner)
	if !ok || r.running == nil {
		return
	}
	ctx, cancel := context.WithCancel(r.running)
	r.runs[name] = cancel
	logger := r.logger.With("workspace", name)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := runner.Run(ctx, logger); err != nil && ctx.Err() == nil {
			logger.ErrorContext(ctx, "workspace stopped", "error", err.Error())
		}
	}()
}

// log logs that the workspace with the name was added, or removed, while
// the registry runs, with what it is: whether it reads, watches, or runs,
// and whether it's available now. The caller holds the lock.
func (r *Registry) log(msg, name string, ws Workspace) {
	if r.logger == nil {
		return
	}
	_, reader := ws.(Reader)
	_, watcher := ws.(Watcher)
	_, runner := ws.(Runner)
	r.logger.Info(msg, "workspace", name, "read", reader, "watch", watcher, "run", runner, "available", available(ws))
}
