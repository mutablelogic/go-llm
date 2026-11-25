package webhook

import (
	"context"
	"log/slog"
	"sync"
	"time"
	"uuid"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// HandleFunc handles the calls of a session's webhooks, such as by running an
// agent with them, and returns nil once they're handled
type HandleFunc func(context.Context, *Calls) error

// Calls are the calls of a session's webhooks, which were accepted, and
// weren't seen, oldest first, and the webhooks they're of
type Calls struct {
	Session  uuid.UUID
	Webhooks []schema.Webhook
	Calls    []schema.Call
}

// handler handles the calls of a webhook server, in one goroutine, once it's
// woken, and marks them as seen
type handler struct {
	client *httpclient.Client
	handle HandleFunc
	logger *slog.Logger
	woken  chan struct{}

	// The sessions whose calls couldn't be handled, until they are, or no
	// longer are tried
	failed map[uuid.UUID]failure

	// The calls which were handled, and weren't marked as seen, as the server
	// couldn't be reached
	unseen []uint64
}

// failure is how many times a session's calls couldn't be handled, and when
// they're tried again
type failure struct {
	count int
	next  time.Time
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// collectWait is how long calls are collected for, from the first, so a burst
// of them is handled together. It's how long calls which couldn't be handled
// wait to be tried again, too, which doubles each time. Tests change it.
var collectWait = 30 * time.Second

const (
	// handleLimit is the most calls which are handled at once, as each has
	// what was sent
	handleLimit = 20

	// handleTries is how many times calls which can't be handled are tried
	handleTries = 5
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func newHandler(client *httpclient.Client, handle HandleFunc, logger *slog.Logger) *handler {
	return &handler{
		client: client,
		handle: handle,
		logger: logger,
		woken:  make(chan struct{}, 1),
		failed: make(map[uuid.UUID]failure),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// wake says that there may be calls to handle, without waiting
func (h *handler) wake() {
	select {
	case h.woken <- struct{}{}:
	default:
	}
}

// run handles the calls, once they've been collected, each time it's woken,
// and those which couldn't be handled, when they're to be tried again, until
// the context is cancelled
func (h *handler) run(ctx context.Context) {
	for {
		var retry <-chan time.Time
		if next := h.next(); !next.IsZero() {
			retry = time.After(time.Until(next))
		}
		select {
		case <-ctx.Done():
			return
		case <-retry:
		case <-h.woken:
			if !sleep(ctx, collectWait) {
				return
			}
		}

		// A call which is received after those which are pending are listed
		// wakes it again
		select {
		case <-h.woken:
		default:
		}
		for {
			err := h.handlePending(ctx)
			if err == nil {
				break
			} else if ctx.Err() != nil {
				return
			}
			h.logger.WarnContext(ctx, "webhook calls not read, which is tried again", "retry", collectWait.String(), "error", err.Error())
			if !sleep(ctx, collectWait) {
				return
			}
		}
	}
}

// next returns when calls which couldn't be handled are first tried again,
// or zero, when there are none
func (h *handler) next() time.Time {
	var next time.Time
	for _, failure := range h.failed {
		if next.IsZero() || failure.next.Before(next) {
			next = failure.next
		}
	}
	return next
}

// handlePending handles the calls which are pending, those of each session
// at the same time, apart from those of the sessions whose calls are to be
// tried again later, and marks them as seen. It returns an error when they
// can't be listed, or marked.
func (h *handler) handlePending(ctx context.Context) error {
	if err := h.see(ctx); err != nil {
		return err
	}
	pending := make(map[uuid.UUID]bool)
	for after := uint64(0); ; {
		list, err := h.client.ListCalls(ctx, schema.CallListRequest{Status: schema.CallPending, After: after, Limit: types.Ptr(uint64(handleLimit))})
		if err != nil {
			return err
		}
		sessions, err := h.sessions(ctx, list.Body)
		if err != nil {
			return err
		}

		// Handle them, and wait until they are
		errs, skipped := make([]error, len(sessions)), make([]bool, len(sessions))
		var wg sync.WaitGroup
		for i, calls := range sessions {
			pending[calls.Session] = true
			if skipped[i] = time.Now().Before(h.failed[calls.Session].next); !skipped[i] {
				wg.Go(func() { errs[i] = h.handle(ctx, calls) })
			}
		}
		wg.Wait()
		for i, calls := range sessions {
			if !skipped[i] {
				h.handled(ctx, calls, errs[i])
			}
		}
		if err := h.see(ctx); err != nil {
			return err
		}
		if len(list.Body) < handleLimit {
			break
		}
		after = list.Body[len(list.Body)-1].ID
	}

	// A session which has no calls which are pending, such as when its
	// webhooks were deleted, isn't tried again
	for session := range h.failed {
		if !pending[session] {
			delete(h.failed, session)
		}
	}
	return nil
}

// sessions returns the calls, by the session which their webhooks are for, in
// the order of their first calls, apart from those whose webhook was deleted
func (h *handler) sessions(ctx context.Context, calls []schema.Call) ([]*Calls, error) {
	var result []*Calls
	sessions := make(map[uuid.UUID]*Calls)
	webhooks := make(map[uint64]*schema.Webhook)
	for _, call := range calls {
		webhook, exists := webhooks[call.Webhook]
		if !exists {
			var err error
			if webhook, err = h.client.GetWebhook(ctx, call.Webhook); isNotFound(err) {
				webhook = nil
			} else if err != nil {
				return nil, err
			}
			webhooks[call.Webhook] = webhook
		}
		if webhook == nil {
			continue
		}
		session := sessions[webhook.Session]
		if session == nil {
			session = &Calls{Session: webhook.Session}
			sessions[webhook.Session] = session
			result = append(result, session)
		}
		if !exists {
			session.Webhooks = append(session.Webhooks, *webhook)
		}
		session.Calls = append(session.Calls, call)
	}
	return result, nil
}

// handled is called once a session's calls were handled, or couldn't be, with
// the error, when they're tried again later, unless they've been tried enough
// times
func (h *handler) handled(ctx context.Context, calls *Calls, err error) {
	if err != nil && ctx.Err() != nil {
		return
	}
	ids := calls.ids()
	attrs := []any{"session", calls.Session.String(), "calls", ids}
	if err != nil {
		failure := h.failed[calls.Session]
		failure.count++
		attrs = append(attrs, "error", err.Error())
		if failure.count < handleTries {
			wait := collectWait << (failure.count - 1)
			failure.next = time.Now().Add(wait)
			h.failed[calls.Session] = failure
			h.logger.WarnContext(ctx, "webhook calls not handled", append(attrs, "retry", wait.String())...)
			return
		}
		h.logger.ErrorContext(ctx, "webhook calls not handled, and not tried again", attrs...)
	} else {
		h.logger.InfoContext(ctx, "webhook calls handled", attrs...)
	}
	delete(h.failed, calls.Session)
	h.unseen = append(h.unseen, ids...)
}

// see marks the calls which were handled as seen, when there are any
func (h *handler) see(ctx context.Context) error {
	if len(h.unseen) == 0 {
		return nil
	} else if _, err := h.client.SeeCalls(ctx, schema.SeenRequest{Calls: h.unseen}); err != nil {
		return err
	}
	h.unseen = nil
	return nil
}

// ids returns the serial numbers of the calls
func (c *Calls) ids() []uint64 {
	result := make([]uint64, 0, len(c.Calls))
	for _, call := range c.Calls {
		result = append(result, call.ID)
	}
	return result
}
