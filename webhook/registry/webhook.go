package registry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	protocol "github.com/mutablelogic/go-llm/webhook/protocol"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// tokenBytes is how many random bytes a webhook's token has, so it
	// can't be guessed
	tokenBytes = 32

	// tokenAttempts is how many tokens are tried, of which only one is a
	// conflict in all likelihood, as they're random
	tokenAttempts = 3
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateWebhook registers a webhook, of the session, and the protocol of its
// type, or generic, when it has none, and returns it, with its token, which
// is in the URL its service calls, which isn't returned again. Returns
// ErrBadParameter when it has no session, or description, or its type isn't
// a protocol.
func (r *Registry) CreateWebhook(ctx context.Context, meta schema.WebhookMeta) (_ schema.WebhookToken, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.CreateWebhook",
		attribute.String("arg.session", meta.Session.String()),
		attribute.String("arg.type", meta.Type),
		attribute.String("arg.description", meta.Description),
	)
	defer func() { endSpan(err) }()

	// It's of a session, and a protocol, by its name
	meta.Description = strings.TrimSpace(meta.Description)
	if meta.Session == uuid.Nil() {
		return schema.WebhookToken{}, llm.ErrBadParameter.With("a webhook needs its session")
	} else if meta.Description == "" {
		return schema.WebhookToken{}, llm.ErrBadParameter.With("a webhook needs its description")
	}
	p, err := protocol.New(meta.Type)
	if err != nil {
		return schema.WebhookToken{}, err
	}
	meta.Type = p.Name()

	// Its token is random, and only its hash is kept
	for range tokenAttempts {
		token, err := newToken()
		if err != nil {
			return schema.WebhookToken{}, err
		}
		webhook, err := r.store.CreateWebhook(ctx, meta, Hash(token))
		if errors.Is(err, llm.ErrConflict) {
			continue
		} else if err != nil {
			return schema.WebhookToken{}, err
		}
		return schema.WebhookToken{Webhook: withoutSecret(webhook), Token: token}, nil
	}
	return schema.WebhookToken{}, llm.ErrInternalServerError.With("no webhook token which isn't another's")
}

// GetWebhook returns the webhook with the serial number, without its secret,
// or ErrNotFound when there's none
func (r *Registry) GetWebhook(ctx context.Context, id uint64) (_ schema.Webhook, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.GetWebhook",
		attribute.Int64("arg.id", int64(id)),
	)
	defer func() { endSpan(err) }()

	webhook, err := r.store.GetWebhook(ctx, id)
	if err != nil {
		return schema.Webhook{}, err
	}
	return withoutSecret(webhook), nil
}

// ListWebhooks returns the webhooks, of the session, when the request has
// one, newest first, without their secrets
func (r *Registry) ListWebhooks(ctx context.Context, req schema.WebhookListRequest) (_ schema.WebhookList, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.ListWebhooks",
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	list, err := r.store.ListWebhooks(ctx, req)
	if err != nil {
		return schema.WebhookList{}, err
	}
	for i := range list.Body {
		list.Body[i] = withoutSecret(list.Body[i])
	}
	return list, nil
}

// UpdateWebhook changes what the request sets, such as the secret, which a
// service gives once it's registered there, such as Pidgeon, and returns
// the webhook, without its secret, or ErrNotFound when there's none, or
// ErrBadParameter when its description is empty
func (r *Registry) UpdateWebhook(ctx context.Context, id uint64, req schema.WebhookRequest) (_ schema.Webhook, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.UpdateWebhook",
		attribute.Int64("arg.id", int64(id)),
		attribute.Bool("arg.description", req.Description != nil),
		attribute.Bool("arg.secret", req.Secret != nil),
	)
	defer func() { endSpan(err) }()

	// A description can be changed, but not removed
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if description == "" {
			return schema.Webhook{}, llm.ErrBadParameter.With("a webhook needs its description")
		}
		req.Description = &description
	}
	webhook, err := r.store.UpdateWebhook(ctx, id, req)
	if err != nil {
		return schema.Webhook{}, err
	}
	return withoutSecret(webhook), nil
}

// DeleteWebhook deletes the webhook, and its calls, so its URL stops
// working at once, and returns it, as it was, without its secret, or
// ErrNotFound when there's none
func (r *Registry) DeleteWebhook(ctx context.Context, id uint64) (_ schema.Webhook, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.DeleteWebhook",
		attribute.Int64("arg.id", int64(id)),
	)
	defer func() { endSpan(err) }()

	webhook, err := r.store.DeleteWebhook(ctx, id)
	if err != nil {
		return schema.Webhook{}, err
	}
	return withoutSecret(webhook), nil
}

// Hash returns the hash of a webhook's token, which it's kept, and looked
// up, by
func Hash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newToken returns a random token, which is in a URL
func newToken() (string, error) {
	data := make([]byte, tokenBytes)
	if _, err := rand.Read(data); err != nil {
		return "", llm.ErrInternalServerError.Withf("webhook token: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// withoutSecret returns the webhook, without its secret, and whether it has
// one
func withoutSecret(webhook schema.Webhook) schema.Webhook {
	webhook.HasSecret = webhook.Secret != ""
	webhook.Secret = ""
	return webhook
}
