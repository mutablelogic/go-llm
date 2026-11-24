package schema

import (
	"strings"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	webhook "github.com/mutablelogic/go-llm/webhook/schema"
	sqlite "github.com/mutablelogic/go-sqlite"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// WebhookID selects a webhook, by its serial number, which is got, updated,
// or deleted
type WebhookID uint64

// WebhookHash selects a webhook, by the hash of its token, which is got
type WebhookHash []byte

// WebhookInsert is a webhook which is created, with the hash of its token
type WebhookInsert struct {
	webhook.WebhookMeta
	Hash []byte
}

// WebhookUpdate is what's changed of a webhook
type WebhookUpdate webhook.WebhookRequest

// WebhookListRequest selects the webhooks which are listed
type WebhookListRequest webhook.WebhookListRequest

// WebhookList is the webhooks which are read, of a list, or one, of a get,
// a create, an update, or a delete
type WebhookList webhook.WebhookList

///////////////////////////////////////////////////////////////////////////////
// SELECTORS

func (id WebhookID) Select(bind *sqlite.Bind, op sqlite.Op) (string, error) {
	bind.Set("id", uint64(id))
	switch op {
	case sqlite.Get:
		return bind.Query("webhook.get"), nil
	case sqlite.Update:
		return bind.Query("webhook.update"), nil
	case sqlite.Delete:
		return bind.Query("webhook.delete"), nil
	}
	return "", sqlite.ErrNotImplemented.Withf("webhook: %v", op)
}

func (hash WebhookHash) Select(bind *sqlite.Bind, op sqlite.Op) (string, error) {
	bind.Set("hash", []byte(hash))
	switch op {
	case sqlite.Get:
		return bind.Query("webhook.get_hash"), nil
	}
	return "", sqlite.ErrNotImplemented.Withf("webhook hash: %v", op)
}

func (req WebhookListRequest) Select(bind *sqlite.Bind, op sqlite.Op) (string, error) {
	if op != sqlite.List {
		return "", sqlite.ErrNotImplemented.Withf("webhooks: %v", op)
	}
	session, err := sessionOf(req.Session)
	if err != nil {
		return "", err
	}
	bind.Set("session", session)
	limit := sqlite.OffsetLimit{Offset: req.Offset, Limit: req.Limit}
	limit.Bind(bind, ListLimit)
	return bind.Query("webhook.list"), nil
}

///////////////////////////////////////////////////////////////////////////////
// WRITERS

// Insert binds the webhook, and returns the query which creates it
func (w WebhookInsert) Insert(bind *sqlite.Bind) (string, error) {
	if w.Session == uuid.Nil() {
		return "", llm.ErrBadParameter.With("a webhook needs its session")
	} else if len(w.Hash) == 0 {
		return "", llm.ErrBadParameter.With("a webhook needs the hash of its token")
	}
	bind.Set("session", w.Session)
	bind.Set("description", strings.TrimSpace(w.Description))
	bind.Set("type", strings.ToLower(strings.TrimSpace(w.Type)))
	bind.Set("secret", w.Secret)
	bind.Set("hash", w.Hash)
	bind.Set("now", time.Now().UTC())
	return bind.Query("webhook.create"), nil
}

func (WebhookInsert) Update(*sqlite.Bind) error {
	return sqlite.ErrNotImplemented.With("a webhook is changed with WebhookUpdate")
}

func (WebhookUpdate) Insert(*sqlite.Bind) (string, error) {
	return "", sqlite.ErrNotImplemented.With("a webhook is created with WebhookInsert")
}

// Update binds what's set, and whether it is
func (u WebhookUpdate) Update(bind *sqlite.Bind) error {
	bind.Set("set_description", u.Description != nil)
	bind.Set("description", strings.TrimSpace(valueOf(u.Description)))
	bind.Set("set_secret", u.Secret != nil)
	bind.Set("secret", valueOf(u.Secret))
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// READERS

func (list *WebhookList) Scan(row sqlite.Row) error {
	var w webhook.Webhook
	if err := row.Scan(&w.ID, &w.Session, &w.Description, &w.Type, &w.Secret, &w.Created); err != nil {
		return err
	}
	list.Body = append(list.Body, w)
	return nil
}

func (list *WebhookList) ScanCount(row sqlite.Row) error {
	return row.Scan(&list.Count)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sessionOf returns the session's identifier, as it's stored, or empty,
// when there's none, or ErrBadParameter, when it isn't one
func sessionOf(session string) (string, error) {
	session = strings.TrimSpace(session)
	if session == "" {
		return "", nil
	}
	id, err := uuid.Parse(session)
	if err != nil {
		return "", llm.ErrBadParameter.Withf("session %q isn't an identifier", session)
	}
	return id.String(), nil
}

// valueOf returns the string, or empty, when there's none
func valueOf(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
