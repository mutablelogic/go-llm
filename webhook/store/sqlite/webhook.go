package sqlite

import (
	"context"
	"errors"

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

func (self *Store) CreateWebhook(ctx context.Context, meta webhook.WebhookMeta, hash []byte) (_ webhook.Webhook, err error) {
	ctx, end := self.span(ctx, "CreateWebhook", attribute.String("arg.session", meta.Session.String()), attribute.String("arg.type", meta.Type))
	defer func() { end(err) }()
	var result schema.WebhookList
	if err := self.conn.Insert(ctx, &result, schema.WebhookInsert{WebhookMeta: meta, Hash: hash}); errors.Is(err, sqlite.ErrConflict) {
		return webhook.Webhook{}, llm.ErrConflict.With("a webhook has the hash of its token")
	} else if err != nil {
		return webhook.Webhook{}, storeErr(err)
	}
	return result.Body[0], nil
}

func (self *Store) GetWebhook(ctx context.Context, id uint64) (_ webhook.Webhook, err error) {
	ctx, end := self.span(ctx, "GetWebhook", attribute.Int64("arg.id", int64(id)))
	defer func() { end(err) }()
	var result schema.WebhookList
	if err := self.conn.Get(ctx, &result, schema.WebhookID(id)); err != nil {
		return webhook.Webhook{}, webhookErr(err, id)
	}
	return result.Body[0], nil
}

func (self *Store) GetWebhookByHash(ctx context.Context, hash []byte) (_ webhook.Webhook, err error) {
	ctx, end := self.span(ctx, "GetWebhookByHash")
	defer func() { end(err) }()
	var result schema.WebhookList
	if err := self.conn.Get(ctx, &result, schema.WebhookHash(hash)); errors.Is(err, sqlite.ErrNotFound) {
		return webhook.Webhook{}, llm.ErrNotFound.With("no webhook has the token")
	} else if err != nil {
		return webhook.Webhook{}, storeErr(err)
	}
	return result.Body[0], nil
}

func (self *Store) ListWebhooks(ctx context.Context, req webhook.WebhookListRequest) (_ webhook.WebhookList, err error) {
	ctx, end := self.span(ctx, "ListWebhooks", attribute.String("arg.req", types.Stringify(req)))
	defer func() { end(err) }()
	result := schema.WebhookList{WebhookListRequest: req}
	if err := self.conn.List(ctx, &result, schema.WebhookListRequest(req)); err != nil {
		return webhook.WebhookList{}, storeErr(err)
	}
	return webhook.WebhookList(result), nil
}

func (self *Store) UpdateWebhook(ctx context.Context, id uint64, req webhook.WebhookRequest) (_ webhook.Webhook, err error) {
	ctx, end := self.span(ctx, "UpdateWebhook", attribute.Int64("arg.id", int64(id)))
	defer func() { end(err) }()
	var result schema.WebhookList
	if err := self.conn.Update(ctx, &result, schema.WebhookID(id), schema.WebhookUpdate(req)); err != nil {
		return webhook.Webhook{}, webhookErr(err, id)
	}
	return result.Body[0], nil
}

func (self *Store) DeleteWebhook(ctx context.Context, id uint64) (_ webhook.Webhook, err error) {
	ctx, end := self.span(ctx, "DeleteWebhook", attribute.Int64("arg.id", int64(id)))
	defer func() { end(err) }()
	var result schema.WebhookList
	if err := self.conn.Delete(ctx, &result, schema.WebhookID(id)); err != nil {
		return webhook.Webhook{}, webhookErr(err, id)
	}
	return result.Body[0], nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// webhookErr returns ErrNotFound, of the webhook, when there's no row, or the
// store's error
func webhookErr(err error, id uint64) error {
	if errors.Is(err, sqlite.ErrNotFound) {
		return llm.ErrNotFound.Withf("webhook %d", id)
	}
	return storeErr(err)
}
