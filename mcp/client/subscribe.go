package client

import (
	"context"
	"slices"
	"sync"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// subscriptions are the resources a session's server acknowledged it
// subscribed to, and those which are waited for, as, with the protocol of
// SEP-2575, a subscription is a subscriptions/listen request, which the SDK
// sends without waiting for the server to say it subscribed
type subscriptions struct {
	mu      sync.Mutex
	acked   map[subscription]bool
	waiting map[subscription][]chan struct{}
}

// subscription is a session's subscription to a resource, by its URI
type subscription struct {
	session *sdkmcp.ClientSession
	uri     string
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// listenProtocol is the first protocol version whose subscriptions are
	// subscriptions/listen requests, which the server acknowledges
	listenProtocol = "2026-07-28"

	// notificationAcked is the server's acknowledgement of a subscription
	notificationAcked = "notifications/subscriptions/acknowledged"

	// ackTimeout is how long a subscription's acknowledgement is waited for
	ackTimeout = 30 * time.Second
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Subscribe asks the server to say when the resource with the URI is
// updated, which WatchResources sends, and returns once it has subscribed,
// so an update after that isn't missed. Returns ErrServiceUnavailable when
// it isn't connected, or the server doesn't acknowledge it subscribed.
func (c *Client) Subscribe(ctx context.Context, uri string) error {
	session, err := c.connected()
	if err != nil {
		return err
	}
	params := &sdkmcp.SubscribeParams{URI: uri}
	if !listens(session) {
		// The server has subscribed, once it responds
		return session.Subscribe(ctx, params)
	}

	// Otherwise wait for it to say it has
	key := subscription{session, uri}
	acked := c.subscriptions.wait(key)
	if acked == nil {
		return nil
	}
	defer c.subscriptions.stopWaiting(key, acked)
	if err := session.Subscribe(ctx, params); err != nil {
		return err
	}
	timeout := time.NewTimer(ackTimeout)
	defer timeout.Stop()
	select {
	case <-acked:
		return nil
	case <-timeout.C:
		return llm.ErrServiceUnavailable.Withf("%s didn't acknowledge the subscription to %q", c.endpoint, uri)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Unsubscribe asks the server to stop saying when the resource with the URI
// is updated. Returns ErrServiceUnavailable when it isn't connected.
func (c *Client) Unsubscribe(ctx context.Context, uri string) error {
	session, err := c.connected()
	if err != nil {
		return err
	}
	c.subscriptions.forget(subscription{session, uri})
	return session.Unsubscribe(ctx, &sdkmcp.UnsubscribeParams{URI: uri})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// acknowledged returns the middleware which says the resources a server
// acknowledged it subscribed to are, to the subscriptions which wait
func (s *subscriptions) acknowledged(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
	return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
		if method == notificationAcked {
			session, _ := req.GetSession().(*sdkmcp.ClientSession)
			if params, ok := req.GetParams().(*sdkmcp.SubscriptionsAcknowledgedParams); ok && session != nil && params != nil {
				for _, uri := range params.Notifications.ResourceSubscriptions {
					s.ack(subscription{session, uri})
				}
			}
		}
		return next(ctx, method, req)
	}
}

// wait returns a channel which is closed once the server acknowledges the
// subscription, or nil, when it has already
func (s *subscriptions) wait(key subscription) chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acked[key] {
		return nil
	}
	if s.waiting == nil {
		s.waiting = make(map[subscription][]chan struct{})
	}
	ch := make(chan struct{})
	s.waiting[key] = append(s.waiting[key], ch)
	return ch
}

// stopWaiting stops waiting for the subscription's acknowledgement with
// the channel, which it may have been
func (s *subscriptions) stopWaiting(key subscription, ch chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waiting[key] = slices.DeleteFunc(s.waiting[key], func(w chan struct{}) bool { return w == ch })
	if len(s.waiting[key]) == 0 {
		delete(s.waiting, key)
	}
}

// ack records the server acknowledged the subscription, and tells those
// which wait
func (s *subscriptions) ack(key subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acked == nil {
		s.acked = make(map[subscription]bool)
	}
	s.acked[key] = true
	for _, ch := range s.waiting[key] {
		close(ch)
	}
	delete(s.waiting, key)
}

// forget forgets the subscription, as it's unsubscribed from
func (s *subscriptions) forget(key subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.acked, key)
}

// forgetSession forgets the session's subscriptions, as it ended
func (s *subscriptions) forgetSession(session *sdkmcp.ClientSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.acked {
		if key.session == session {
			delete(s.acked, key)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// listens returns true when the session's subscriptions are
// subscriptions/listen requests, which the server acknowledges
func listens(session *sdkmcp.ClientSession) bool {
	result := session.InitializeResult()
	return result != nil && result.ProtocolVersion >= listenProtocol
}
