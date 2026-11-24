// Package store is where webhooks are kept, such as in files, or a
// database, so they're kept when the server restarts
package store

import (
	"context"
	"time"

	// Packages
	schema "github.com/mutablelogic/go-llm/webhook/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The schema, and tables, of a store's changes, whose rows, which are their
// serial numbers, are published to a broadcaster, as they're created,
// updated and deleted, by a store which publishes them
const (
	Schema       = "webhook"
	WebhookTable = "webhook"
	CallTable    = "call"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store keeps the webhooks, and their calls
type Store interface {
	WebhookStore
	CallStore
}

// WebhookStore keeps the webhooks: what each is for, its secret, which its
// calls are verified with, and the hash of its token, which is in the URL
// its service calls, rather than the token, so a call is looked up by its
// token's hash, and the token can't be read from the store
type WebhookStore interface {
	// CreateWebhook adds a webhook, with the hash of its token, and returns
	// it, with the serial number it's given, and when it was created, or
	// ErrConflict when there's one with the hash
	CreateWebhook(ctx context.Context, meta schema.WebhookMeta, hash []byte) (schema.Webhook, error)

	// GetWebhook returns the webhook with the serial number, or ErrNotFound
	// when there's none
	GetWebhook(ctx context.Context, id uint64) (schema.Webhook, error)

	// GetWebhookByHash returns the webhook whose token has the hash, which
	// a call is made with, or ErrNotFound when there's none
	GetWebhookByHash(ctx context.Context, hash []byte) (schema.Webhook, error)

	// ListWebhooks returns the webhooks, those of the session, when the
	// request has one, from its offset, and at most its limit of them,
	// newest first, and how many there are
	ListWebhooks(ctx context.Context, req schema.WebhookListRequest) (schema.WebhookList, error)

	// UpdateWebhook changes what the request sets, such as the description,
	// or the secret, which the service gives, once it's registered there,
	// and returns the webhook, or ErrNotFound when there's none
	UpdateWebhook(ctx context.Context, id uint64, req schema.WebhookRequest) (schema.Webhook, error)

	// DeleteWebhook deletes the webhook, and its calls, and returns it, as
	// it was, so its URL stops working at once, or ErrNotFound when there's
	// none
	DeleteWebhook(ctx context.Context, id uint64) (schema.Webhook, error)
}

// CallStore keeps the calls of the webhooks, as they were received, which
// are listed after a call, by its serial number, so what watches them can
// carry on from the last it had, such as when the server restarts
type CallStore interface {
	// CreateCall adds the call of its webhook, and returns it, with the
	// serial number it's given, which is greater than any call's before it,
	// or ErrNotFound when there's no webhook, or ErrConflict when the
	// webhook has a call with its delivery, when it has one, which is a
	// delivery which was sent again, of the calls which weren't refused
	CreateCall(ctx context.Context, call schema.Call) (schema.Call, error)

	// GetCall returns the call with the serial number, or ErrNotFound when
	// there's none
	GetCall(ctx context.Context, id uint64) (schema.Call, error)

	// ListCalls returns the calls, those of the webhook, or of the session's
	// webhooks, when the request has one, after the call with its serial
	// number, and received since its time, oldest first, or newest, when
	// the request says so, and at most its limit of them, and how many
	// there are, which were accepted, or refused, or both, of the request's
	// status
	ListCalls(ctx context.Context, req schema.CallListRequest) (schema.CallList, error)

	// DeleteCall deletes the call with the serial number, and returns it, as
	// it was, or ErrNotFound when there's none
	DeleteCall(ctx context.Context, id uint64) (schema.Call, error)

	// DeleteCalls deletes the calls which were received before the time,
	// and returns how many were deleted
	DeleteCalls(ctx context.Context, before time.Time) (uint64, error)
}
