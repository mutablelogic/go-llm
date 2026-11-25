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

// handler handles the calls of a webhook server, once it's woken, and marks
// them as seen, those of each session in a goroutine of its own, so a session
// doesn't wait for another's
type handler struct {
	client *httpclient.Client
	handle HandleFunc
	logger *slog.Logger
	woken  chan struct{}
	slots  chan struct{}
	wg     sync.WaitGroup

	// The sessions whose calls are being handled, and whether more may have
	// been received since
	mu     sync.Mutex
	active map[uuid.UUID]bool
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// collectWait is how long calls are collected for, from the first, so a burst
// of them is handled together. It's how long those which couldn't be handled
// wait to be tried again, too, which doubles each time. Tests change it.
var collectWait = 30 * time.Second

const (
	// handleLimit is the most calls which are handled at once, as each has
	// what was sent
	handleLimit = 20

	// handleSlots is the most sessions whose calls are handled at once
	handleSlots = 4

	// handleTries is how many times a call which can't be handled is tried
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
		slots:  make(chan struct{}, handleSlots),
		active: make(map[uuid.UUID]bool),
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
// until the context is cancelled, when it returns once those which are being
// handled are
func (h *handler) run(ctx context.Context) {
	defer h.wg.Wait()
	for {
		select {
		case <-ctx.Done():
			return
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
			err := h.start(ctx)
			if err == nil {
				break
			} else if !h.retry(ctx, err) {
				return
			}
		}
	}
}

// retry logs that calls couldn't be read, or marked as seen, and waits to try
// again, and returns false when the context is cancelled first
func (h *handler) retry(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	h.logger.WarnContext(ctx, "webhook calls not read, which is tried again", "retry", collectWait.String(), "error", err.Error())
	return sleep(ctx, collectWait)
}

// start starts handling the calls of the sessions which have calls which are
// pending, apart from those whose calls are being handled, which are read
// again
func (h *handler) start(ctx context.Context) error {
	webhooks := make(map[uint64]*schema.Webhook)
	started := make(map[uuid.UUID]bool)
	for after := uint64(0); ; {
		list, err := h.client.ListCalls(ctx, schema.CallListRequest{Status: schema.CallPending, After: after, Limit: types.Ptr(uint64(handleLimit))})
		if err != nil {
			return err
		}
		for _, call := range list.Body {
			if webhook, err := h.webhook(ctx, webhooks, call.Webhook); err != nil {
				return err
			} else if webhook != nil && !started[webhook.Session] {
				started[webhook.Session] = true
				h.startSession(ctx, webhook.Session)
			}
		}
		if len(list.Body) < handleLimit {
			return nil
		}
		after = list.Body[len(list.Body)-1].ID
	}
}

// startSession starts handling the session's calls, unless they're being
// handled, when they're read again, once those are
func (h *handler) startSession(ctx context.Context, session uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, active := h.active[session]; active {
		h.active[session] = true
		return
	}
	h.active[session] = false
	h.wg.Go(func() { h.handleSession(ctx, session) })
}

// stopSession returns true when the session has no more calls, as none may
// have been received since they were read, so it's no longer handled
func (h *handler) stopSession(session uuid.UUID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active[session] {
		h.active[session] = false
		return false
	}
	delete(h.active, session)
	return true
}

// handleSession handles the session's calls which are pending, oldest first,
// and marks them as seen, until it has none, or the context is cancelled.
// Those which can't be handled are tried again, with those received since,
// waiting longer each time, until each has been tried enough times.
func (h *handler) handleSession(ctx context.Context, session uuid.UUID) {
	var seen []uint64
	tries := make(map[uint64]int)
	for {
		calls, err := h.read(ctx, session, seen)
		if err != nil {
			if !h.retry(ctx, err) {
				return
			}
			continue
		}
		seen = nil
		if len(calls.Calls) == 0 {
			if h.stopSession(session) {
				return
			}
			continue
		}

		// Handle them, once there's a slot
		select {
		case h.slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		err = h.handle(ctx, calls)
		<-h.slots
		if err != nil && ctx.Err() != nil {
			return
		}
		var wait time.Duration
		if seen, wait = h.handled(ctx, calls, err, tries); wait > 0 && !sleep(ctx, wait) {
			return
		}
	}
}

// read marks the calls which were handled as seen, when there are any, and
// returns the session's which are pending, and the webhooks they're of, apart
// from those whose webhook was deleted
func (h *handler) read(ctx context.Context, session uuid.UUID, seen []uint64) (*Calls, error) {
	if len(seen) > 0 {
		if _, err := h.client.SeeCalls(ctx, schema.SeenRequest{Calls: seen}); err != nil {
			return nil, err
		}
	}
	list, err := h.client.ListCalls(ctx, schema.CallListRequest{Session: session.String(), Status: schema.CallPending, Limit: types.Ptr(uint64(handleLimit))})
	if err != nil {
		return nil, err
	}
	result := &Calls{Session: session}
	webhooks := make(map[uint64]*schema.Webhook)
	for _, call := range list.Body {
		_, exists := webhooks[call.Webhook]
		webhook, err := h.webhook(ctx, webhooks, call.Webhook)
		if err != nil {
			return nil, err
		} else if webhook == nil {
			continue
		}
		if !exists {
			result.Webhooks = append(result.Webhooks, *webhook)
		}
		result.Calls = append(result.Calls, call)
	}
	return result, nil
}

// webhook returns the webhook with the serial number, which is added to the
// webhooks, or nil, when it was deleted
func (h *handler) webhook(ctx context.Context, webhooks map[uint64]*schema.Webhook, id uint64) (*schema.Webhook, error) {
	if webhook, exists := webhooks[id]; exists {
		return webhook, nil
	}
	webhook, err := h.client.GetWebhook(ctx, id)
	if isNotFound(err) {
		webhook = nil
	} else if err != nil {
		return nil, err
	}
	webhooks[id] = webhook
	return webhook, nil
}

// handled returns the calls which are to be marked as seen, as they were
// handled, or, with the error, as they've been tried enough times, and how
// long to wait to try the rest again, when there are any
func (h *handler) handled(ctx context.Context, calls *Calls, err error, tries map[uint64]int) (seen []uint64, wait time.Duration) {
	attrs := []any{"session", calls.Session.String()}
	if err == nil {
		for _, id := range calls.ids() {
			delete(tries, id)
		}
		h.logger.InfoContext(ctx, "webhook calls handled", append(attrs, "calls", calls.ids())...)
		return calls.ids(), 0
	}
	attrs = append(attrs, "error", err.Error())
	var again []uint64
	most := 0
	for _, id := range calls.ids() {
		if tries[id]++; tries[id] >= handleTries {
			delete(tries, id)
			seen = append(seen, id)
		} else {
			again, most = append(again, id), max(most, tries[id])
		}
	}
	if len(seen) > 0 {
		h.logger.ErrorContext(ctx, "webhook calls not handled, and not tried again", append(attrs, "calls", seen)...)
	}
	if len(again) > 0 {
		wait = collectWait << (most - 1)
		h.logger.WarnContext(ctx, "webhook calls not handled", append(attrs, "calls", again, "retry", wait.String())...)
	}
	return seen, wait
}

// ids returns the serial numbers of the calls
func (c *Calls) ids() []uint64 {
	result := make([]uint64, 0, len(c.Calls))
	for _, call := range c.Calls {
		result = append(result, call.ID)
	}
	return result
}
