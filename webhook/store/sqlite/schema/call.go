package schema

import (
	"encoding/json"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	webhook "github.com/mutablelogic/go-llm/webhook/schema"
	sqlite "github.com/mutablelogic/go-sqlite"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// CallID selects a call, by its serial number, which is got, or deleted
type CallID uint64

// CallIDs selects calls, by their serial numbers, which are marked as seen
type CallIDs []uint64

// CallSeen marks calls as seen, at the time
type CallSeen time.Time

// CallInsert is a call which is created
type CallInsert webhook.Call

// CallListRequest selects the calls which are listed
type CallListRequest webhook.CallListRequest

// CallList is the calls which are read, of a list, or one, of a get, a
// create, or a delete
type CallList webhook.CallList

// CallsBefore selects the calls received before the time, which are
// deleted
type CallsBefore time.Time

// CallCount is how many calls were deleted, or marked as seen
type CallCount uint64

///////////////////////////////////////////////////////////////////////////////
// SELECTORS

func (id CallID) Select(bind *sqlite.Bind, op sqlite.Op) (string, error) {
	bind.Set("id", uint64(id))
	switch op {
	case sqlite.Get:
		return bind.Query("call.get"), nil
	case sqlite.Delete:
		return bind.Query("call.delete"), nil
	}
	return "", sqlite.ErrNotImplemented.Withf("call: %v", op)
}

func (ids CallIDs) Select(bind *sqlite.Bind, op sqlite.Op) (string, error) {
	if op != sqlite.Update {
		return "", sqlite.ErrNotImplemented.Withf("calls: %v", op)
	} else if len(ids) > ListLimit {
		return "", llm.ErrBadParameter.Withf("%d calls are more than %d", len(ids), ListLimit)
	}
	data, err := json.Marshal([]uint64(ids))
	if err != nil {
		return "", err
	}
	bind.Set("ids", string(data))
	return bind.Query("call.seen"), nil
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
	case "", webhook.CallAccepted, webhook.CallRefused, webhook.CallPending, webhook.CallSeen:
	default:
		return "", llm.ErrBadParameter.Withf("call status %q isn't %s, %s, %s or %s", req.Status, webhook.CallAccepted, webhook.CallRefused, webhook.CallPending, webhook.CallSeen)
	}
	bind.Set("status", status)
	bind.Set("since", sinceOf(req.Since))
	bind.Set("order", "ASC")
	if req.Newest {
		bind.Set("order", "DESC")
	}
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
	bind.Set("test", c.Test)
	bind.Set("error", c.Error)
	bind.Set("body", nilIfEmpty(len(c.Body) == 0, c.Body))
	return bind.Query("call.create"), nil
}

func (CallInsert) Update(*sqlite.Bind) error {
	return sqlite.ErrNotImplemented.With("a call isn't changed")
}

func (CallSeen) Insert(*sqlite.Bind) (string, error) {
	return "", sqlite.ErrNotImplemented.With("a call is created with CallInsert")
}

// Update binds when the calls were seen
func (seen CallSeen) Update(bind *sqlite.Bind) error {
	bind.Set("now", time.Time(seen).UTC())
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// READERS

func (list *CallList) Scan(row sqlite.Row) error {
	var c webhook.Call
	if err := row.Scan(&c.ID, &c.Webhook, &c.Received, &c.Method, &c.Query, &c.Headers, &c.Values, &c.Event, &c.Delivery, &c.Verified, &c.Test, &c.Error, &c.Seen, &c.Body); err != nil {
		return err
	}
	list.Body = append(list.Body, c)
	return nil
}

func (list *CallList) ScanCount(row sqlite.Row) error {
	return row.Scan(&list.Count)
}

// Scan counts a call which was deleted, or marked as seen
func (count *CallCount) Scan(sqlite.Row) error {
	*count++
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sinceOf returns the time, to the second, which the times which are stored,
// in that second, or after, sort after, or the same as, or empty, which every
// time sorts after, when it's zero. They're stored with as many decimals as
// they have, so a time with a fraction, or a time zone, doesn't sort as it
// should against one which has fewer.
func sinceOf(since time.Time) string {
	if since.IsZero() {
		return ""
	}
	return since.UTC().Format("2006-01-02T15:04:05")
}

// nilIfEmpty returns nil, which is NULL, when it's empty, or the value
func nilIfEmpty(empty bool, value any) any {
	if empty {
		return nil
	}
	return value
}
