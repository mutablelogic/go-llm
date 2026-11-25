package webhook

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	store "github.com/mutablelogic/go-llm/webhook/store"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// retryMin and retryMax are how long to wait before watching again, which
// doubles each time, until a watch starts. Tests change them.
var (
	retryMin = time.Second
	retryMax = time.Minute
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Handle sets the function which handles the calls of a session's webhooks,
// such as by running an agent with them. It's set before the namespace runs.
func (ns *Namespace) Handle(fn HandleFunc) {
	ns.running.Lock()
	defer ns.running.Unlock()
	ns.handle = fn
}

// Run watches the webhook server for the calls of its webhooks, and, with
// Handle, handles those which were accepted, and weren't seen, which include
// those received while it wasn't running, until the context is cancelled,
// when it returns nil. Those received within half a minute of the first are
// handled together, those of each session at once. They're marked as seen
// once they're handled, so they're not handled again, unless it stops before
// they're marked.
//
// When the server can't be reached, or stops, it tries again, waiting longer
// each time. Calls which can't be handled are tried again, up to a few
// times, waiting longer each time, and then marked as seen, so they don't
// stop those after them. What was sent isn't logged. Returns ErrConflict when
// it's running already.
func (ns *Namespace) Run(parent context.Context, logger *slog.Logger) error {
	if !ns.running.TryLock() {
		return llm.ErrConflict.Withf("namespace %q is running", Name)
	}
	defer ns.running.Unlock()
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	logger = logger.With("namespace", Name)
	if ns.handle == nil {
		<-parent.Done()
		return nil
	}

	// Handle the calls, each time the watch says that there may be some, or
	// that it started, and stop, once it has
	ctx, cancel := context.WithCancel(parent)
	h := newHandler(ns.client, ns.handle, logger)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.run(ctx)
	}()
	defer func() {
		cancel()
		<-done
	}()

	wait := retryMin
	for {
		started, err := ns.watchCalls(ctx, logger, h.wake)
		if ctx.Err() != nil {
			return nil
		}
		if started {
			wait = retryMin
		}
		attrs := []any{"retry", wait.String()}
		if err != nil {
			attrs = append(attrs, "error", err.Error())
		}
		logger.WarnContext(ctx, "webhook calls aren't watched", attrs...)
		if !sleep(ctx, wait) {
			return nil
		}
		wait = min(wait*2, retryMax)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// watchCalls watches for calls, until the watch ends, and returns whether it
// started, and why it ended. The function is called once it starts, as calls
// may have been missed, and as each call is received, or changes are missed.
func (ns *Namespace) watchCalls(ctx context.Context, logger *slog.Logger, wake func()) (started bool, err error) {
	err = ns.client.DoWithContext(ctx, client.MethodGet, nil,
		client.OptPath("changes"),
		client.OptQuery(eventschema.ChangeFilter{Schema: store.Schema, Table: store.CallTable, Action: eventschema.ChangeCreate}.Query()),
		client.OptReqHeader("Accept", types.ContentTypeTextStream),
		client.OptNoTimeout(),
		client.OptResponse(func(resp *http.Response) error {
			if resp.StatusCode == http.StatusOK {
				started = true
				logger.InfoContext(ctx, "webhook calls are watched")
				wake()
			}
			return nil
		}),
		client.OptTextStreamCallback(func(event client.TextStreamEvent) error {
			var change eventschema.Change
			if event.Event != eventschema.ChangeName {
				return nil
			} else if err := event.Json(&change); err != nil {
				return err
			}
			if change.Schema == store.Schema && change.Table == store.CallTable {
				wake()
			}
			return nil
		}),
	)
	return started, err
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sleep waits for the time, and returns true, or false, when the context is
// cancelled first
func sleep(ctx context.Context, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// isNotFound returns whether the error is a response whose status is 404
func isNotFound(err error) bool {
	var response httpresponse.ErrResponse
	if errors.As(err, &response) {
		return response.Code == http.StatusNotFound
	}
	var status httpresponse.Err
	return errors.As(err, &status) && int(status) == http.StatusNotFound
}
