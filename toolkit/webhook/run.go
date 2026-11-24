package webhook

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	webhookschema "github.com/mutablelogic/go-llm/webhook/schema"
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

const (
	// eventBuffer is how many changes are kept, until they're handled
	eventBuffer = 256

	// missedLimit is how many calls which were missed are read at once, as
	// each has what was sent, which can be several megabytes
	missedLimit = 10
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run watches the webhook server for the calls of its webhooks, and logs
// each to the logger, until the context is cancelled, when it returns nil. It
// says whether the call was accepted, or refused, but not what was sent.
//
// When the server can't be reached, or stops, it tries again, waiting longer
// each time, and then logs the calls which it missed, after the last which
// it logged, as it does when it's run again. Those which were received before
// the first it's told of aren't. Returns ErrConflict when it's running
// already.
func (ns *Namespace) Run(ctx context.Context, logger *slog.Logger) error {
	if !ns.running.TryLock() {
		return llm.ErrConflict.Withf("namespace %q is running", Name)
	}
	defer ns.running.Unlock()
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	logger = logger.With("namespace", Name)

	wait := retryMin
	for {
		started, err := ns.watchCalls(ctx, logger)
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
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		wait = min(wait*2, retryMax)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// watchCalls watches for calls, until the watch ends, or a call can't be read,
// and returns whether it started, and why it ended
func (ns *Namespace) watchCalls(parent context.Context, logger *slog.Logger) (bool, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	// Read the calls which are created, as they're sent, and say when the
	// server is watching, which it is once it responds
	started, changes, done := make(chan struct{}), make(chan eventschema.Change, eventBuffer), make(chan error, 1)
	go func() {
		done <- ns.client.DoWithContext(ctx, client.MethodGet, nil,
			client.OptPath("changes"),
			client.OptQuery(eventschema.ChangeFilter{Schema: store.Schema, Table: store.CallTable, Action: eventschema.ChangeCreate}.Query()),
			client.OptReqHeader("Accept", types.ContentTypeTextStream),
			client.OptNoTimeout(),
			client.OptResponse(func(resp *http.Response) error {
				if resp.StatusCode == http.StatusOK {
					close(started)
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
				select {
				case changes <- change:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}),
		)
	}()

	// Once it's watching, handle the calls which were missed, so none is
	// between them and those which are watched
	select {
	case <-started:
	case err := <-done:
		return false, err
	}
	logger.InfoContext(ctx, "webhook calls are watched")
	ns.missed = 0
	if err := ns.handleMissed(ctx, logger); err != nil {
		return true, err
	}

	// Handle the calls which are watched, and those which were sent before
	// the watch ended
	for {
		select {
		case change := <-changes:
			if err := ns.handleChange(ctx, logger, change); err != nil {
				return true, err
			}
		case err := <-done:
			for {
				select {
				case change := <-changes:
					if err := ns.handleChange(ctx, logger, change); err != nil {
						return true, err
					}
				default:
					return true, err
				}
			}
		}
	}
}

// handleChange handles the call which was created, unless it was handled
// already, as it was missed, or those which were missed, when changes were,
// which is a change without a row
func (ns *Namespace) handleChange(ctx context.Context, logger *slog.Logger, change eventschema.Change) error {
	if change.Schema != store.Schema || change.Table != store.CallTable {
		return nil
	} else if change.Row == "" {
		return ns.handleMissed(ctx, logger)
	} else if change.Action != eventschema.ChangeCreate {
		return nil
	}
	id, err := strconv.ParseUint(change.Row, 10, 64)
	if err != nil || id == 0 || id <= ns.missed {
		return nil
	}

	// The first call which is seen is missed, when it can't be read
	if !ns.seen {
		ns.seen, ns.last = true, id-1
	}

	// A call which was deleted, since it was received, isn't handled
	call, err := ns.client.GetCall(ctx, id)
	if isNotFound(err) {
		ns.last = id
		return nil
	} else if err != nil {
		return err
	}
	return ns.handle(ctx, logger, *call)
}

// handleMissed handles the calls after the last which was handled, when one
// was seen
func (ns *Namespace) handleMissed(ctx context.Context, logger *slog.Logger) error {
	for ns.seen {
		list, err := ns.client.ListCalls(ctx, webhookschema.CallListRequest{After: ns.last, Limit: types.Ptr(uint64(missedLimit))})
		if err != nil {
			return err
		}
		for _, call := range list.Body {
			if err := ns.handle(ctx, logger, call); err != nil {
				return err
			}
			ns.missed = call.ID
		}
		if len(list.Body) < missedLimit {
			break
		}
	}
	return nil
}

// handle logs the call, with its webhook, and its session, unless the
// webhook was deleted
func (ns *Namespace) handle(ctx context.Context, logger *slog.Logger, call webhookschema.Call) error {
	attrs := []any{"call", call.ID, "webhook", call.Webhook}
	if webhook, err := ns.client.GetWebhook(ctx, call.Webhook); err == nil {
		attrs = append(attrs, "description", webhook.Description, "session", webhook.Session.String())
	} else if !isNotFound(err) {
		return err
	}
	if call.Event != "" {
		attrs = append(attrs, "event", call.Event)
	}
	if call.Delivery != "" {
		attrs = append(attrs, "delivery", call.Delivery)
	}
	attrs = append(attrs, "verified", call.Verified)
	if call.Error != "" {
		logger.WarnContext(ctx, "webhook call refused", append(attrs, "error", call.Error)...)
	} else {
		logger.InfoContext(ctx, "webhook call", attrs...)
	}
	ns.last = call.ID
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// isNotFound returns whether the error is a response whose status is 404
func isNotFound(err error) bool {
	var response httpresponse.ErrResponse
	if errors.As(err, &response) {
		return response.Code == http.StatusNotFound
	}
	var status httpresponse.Err
	return errors.As(err, &status) && int(status) == http.StatusNotFound
}
