//go:build !client

package main

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	memoryschema "github.com/mutablelogic/go-llm/memory/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	registryschema "github.com/mutablelogic/go-llm/registry/schema"
	webhook "github.com/mutablelogic/go-llm/toolkit/webhook"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// handleWebhooks returns the function which handles the calls of a session's
// webhooks, with the webhook agent, which is run as an event of the session,
// in an event session of its webhooks, and with the session's tools, apart
// from the heartbeats', and the webhooks', so a call doesn't schedule events,
// or make calls, and whose reply is delivered to the session
func handleWebhooks(events *atomic.Pointer[registry.Registry], details func() memoryschema.Details, logger *slog.Logger) webhook.HandleFunc {
	return func(ctx context.Context, calls *webhook.Calls) error {
		r := events.Load()
		if r == nil {
			return llm.ErrServiceUnavailable.With("registry isn't running")
		}
		now := time.Now()
		input, err := calls.Input(ctx, eventContext(now, details()))
		if err != nil {
			return err
		}
		response, err := r.RunEvent(ctx, registryschema.EventRequest{Session: calls.Session, Source: webhook.Name}.RunID(), registryschema.RunEventRequest{
			Session: calls.Session,
			Agent:   webhook.Agent,
			Input:   input,
			Without: []string{heartbeat.Name, webhook.Name},
			Title:   "Webhooks",
			Source:  webhook.Name,
			Fired:   now,
		}, nil)
		if err != nil {
			return err
		}
		logger.InfoContext(ctx, "webhook agent run", "session", calls.Session, "calls", len(calls.Calls), "event", response.Session.ID, "model", types.Value(response.Session.Model))
		return nil
	}
}
