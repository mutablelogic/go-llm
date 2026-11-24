package schema

import (
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	webhook "github.com/mutablelogic/go-llm/webhook/schema"
	sqlite "github.com/mutablelogic/go-sqlite"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// CallID selects a call, by its serial number, which is got
type CallID uint64

// CallInsert is a call which is created
type CallInsert webhook.Call

// CallListRequest selects the calls which are listed
type CallListRequest webhook.CallListRequest

// CallList is the calls which are read, of a list, or one, of a get, or a
// create
type CallList webhook.CallList

// CallsBefore selects the calls received before the time, which are
// deleted
type CallsBefore time.Time

// CallCount is how many calls were deleted
type CallCount uint64

///////////////////////////////////////////////////////////////////////////////
// SELECTORS

func (id CallID) Select(bind *sqlite.Bind, op sqlite.Op) (string, error) {
	bind.Set("id", uint64(id))
	switch op {
	case sqlite.Get:
		return bind.Query("call.get"), nil
	}
	return "", sqlite.ErrNotImplemented.Withf("call: %v", op)
}

func (req CallListRequest) Select(bind *sqlite.Bind, op sqlite.Op) (string, error) {
	if op != sqlite.List {
		return "", sqlite.ErrNotImplemented.Withf("calls: %v", op)
	}
	session, err := sessionOf(req.Session)
	if err != nil {
		return "", err
	}
	bind.Set("webhook", req.Webhook)
	bind.Set("session", session)
	bind.Set("after", req.After)
	status := strings.ToLower(strings.TrimSpace(req.Status))
	switch status {
	case "", webhook.CallAccepted, webhook.CallRefused:
	default:
		return "", llm.ErrBadParameter.Withf("call status %q isn't %s, or %s", req.Status, webhook.CallAccepted, webhook.CallRefused)
	}
	bind.Set("status", status)
	limit := sqlite.OffsetLimit{Limit: req.Limit}
	limit.Bind(bind, ListLimit)
	return bind.Query("call.list"), nil
}

func (before CallsBefore) Select(bind *sqlite.Bind, op sqlite.Op) (string, error) {
	bind.Set("before", time.Time(before).UTC())
	switch op {
	case sqlite.Delete:
		return bind.Query("call.delete_before"), nil
	}
	return "", sqlite.ErrNotImplemented.Withf("calls before: %v", op)
}

///////////////////////////////////////////////////////////////////////////////
// WRITERS

// Insert binds the call, which is received now, when it has no time, and
// returns the query which creates it
func (c CallInsert) Insert(bind *sqlite.Bind) (string, error) {
	if c.Webhook == 0 {
		return "", llm.ErrBadParameter.With("a call needs its webhook")
	} else if c.Method == "" {
		return "", llm.ErrBadParameter.With("a call needs its method")
	}
	received := c.Received
	if received.IsZero() {
		received = time.Now()
	}
	bind.Set("webhook", c.Webhook)
	bind.Set("received", received.UTC())
	bind.Set("method", c.Method)
	bind.Set("query", nilIfEmpty(len(c.Query) == 0, c.Query))
	bind.Set("headers", nilIfEmpty(len(c.Headers) == 0, c.Headers))
	bind.Set("values", nilIfEmpty(len(c.Values) == 0, c.Values))
	bind.Set("event", c.Event)
	bind.Set("delivery", c.Delivery)
	bind.Set("verified", c.Verified)
	bind.Set("error", c.Error)
	bind.Set("body", nilIfEmpty(len(c.Body) == 0, c.Body))
	return bind.Query("call.create"), nil
}

func (CallInsert) Update(*sqlite.Bind) error {
	return sqlite.ErrNotImplemented.With("a call isn't changed")
}

///////////////////////////////////////////////////////////////////////////////
// READERS

func (list *CallList) Scan(row sqlite.Row) error {
	var c webhook.Call
	if err := row.Scan(&c.ID, &c.Webhook, &c.Received, &c.Method, &c.Query, &c.Headers, &c.Values, &c.Event, &c.Delivery, &c.Verified, &c.Error, &c.Body); err != nil {
		return err
	}
	list.Body = append(list.Body, c)
	return nil
}

func (list *CallList) ScanCount(row sqlite.Row) error {
	return row.Scan(&list.Count)
}

// Scan counts a call which was deleted
func (count *CallCount) Scan(sqlite.Row) error {
	*count++
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// nilIfEmpty returns nil, which is NULL, when it's empty, or the value
func nilIfEmpty(empty bool, value any) any {
	if empty {
		return nil
	}
	return value
}
