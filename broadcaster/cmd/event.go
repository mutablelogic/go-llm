package cmd

import (
	"context"
	"encoding/json"
	"os"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/broadcaster/httpclient"
	schema "github.com/mutablelogic/go-llm/broadcaster/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// EventCommands are the commands which watch the broadcaster's events
type EventCommands struct {
	Events EventsCmd `cmd:"" name:"events" help:"Watch the changes to providers, sessions, tools, memory and heartbeats." group:"EVENTS"`
}

type EventsCmd struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints each event as a line of JSON, until the command is interrupted
func (cmd *EventsCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.EventsCmd", func(ctx context.Context, client *httpclient.Client) error {
		encoder := json.NewEncoder(os.Stdout)
		return client.WatchEvents(ctx, func(event schema.Event) error {
			return encoder.Encode(event)
		})
	})
}
