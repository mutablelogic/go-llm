package sqlite

import (
	"context"
	"errors"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	webhook "github.com/mutablelogic/go-llm/webhook/schema"
	schema "github.com/mutablelogic/go-llm/webhook/store/sqlite/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	sqlite "github.com/mutablelogic/go-sqlite"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateCall adds the call, once its webhook is known to be there, in a
// transaction, so it isn't deleted between them
func (self *Store) CreateCall(ctx context.Context, call webhook.Call) (_ webhook.Call, err error) {
	ctx, end := self.span(ctx, "CreateCall", attribute.Int64("arg.webhook", int64(call.Webhook)), attribute.String("arg.event", call.Event))
	defer func() { end(err) }()
	var result schema.CallList
	if err := self.conn.Tx(ctx, func(conn sqlite.Conn) error {
		var hook schema.WebhookList
		if err := conn.Get(ctx, &hook, schema.WebhookID(call.Webhook)); err != nil {
			return webhookErr(err, call.Webhook)
		}
		return conn.Insert(ctx, &result, schema.CallInsert(call))
	}); errors.Is(err, sqlite.ErrConflict) {
		return webhook.Call{}, llm.ErrConflict.Withf("webhook %d has a call of delivery %q", call.Webhook, call.Delivery)
	} else if err != nil {
		return webhook.Call{}, storeErr(err)
	}
	return result.Body[0], nil
}

func (self *Store) GetCall(ctx context.Context, id uint64) (_ webhook.Call, err error) {
	ctx, end := self.span(ctx, "GetCall", attribute.Int64("arg.id", int64(id)))
	defer func() { end(err) }()
	var result schema.CallList
	if err := self.conn.Get(ctx, &result, schema.CallID(id)); errors.Is(err, sqlite.ErrNotFound) {
		return webhook.Call{}, llm.ErrNotFound.Withf("call %d", id)
	} else if err != nil {
		return webhook.Call{}, storeErr(err)
	}
	return result.Body[0], nil
}

func (self *Store) DeleteCall(ctx context.Context, id uint64) (_ webhook.Call, err error) {
	ctx, end := self.span(ctx, "DeleteCall", attribute.Int64("arg.id", int64(id)))
	defer func() { end(err) }()
	var result schema.CallList
	if err := self.conn.Delete(ctx, &result, schema.CallID(id)); errors.Is(err, sqlite.ErrNotFound) {
		return webhook.Call{}, llm.ErrNotFound.Withf("call %d", id)
	} else if err != nil {
		return webhook.Call{}, storeErr(err)
	}
	return result.Body[0], nil
}

func (self *Store) ListCalls(ctx context.Context, req webhook.CallListRequest) (_ webhook.CallList, err error) {
	ctx, end := self.span(ctx, "ListCalls", attribute.String("arg.req", types.Stringify(req)))
	defer func() { end(err) }()
	result := schema.CallList{CallListRequest: req}
	if err := self.conn.List(ctx, &result, schema.CallListRequest(req)); err != nil {
		return webhook.CallList{}, storeErr(err)
	}
	return webhook.CallList(result), nil
}

func (self *Store) DeleteCalls(ctx context.Context, before time.Time) (_ uint64, err error) {
	ctx, end := self.span(ctx, "DeleteCalls", attribute.String("arg.before", before.UTC().Format(time.RFC3339)))
	defer func() { end(err) }()
	var count schema.CallCount
	if err := self.conn.Delete(ctx, &count, schema.CallsBefore(before)); err != nil && !errors.Is(err, sqlite.ErrNotFound) {
		return 0, storeErr(err)
	}
	return uint64(count), nil
}
