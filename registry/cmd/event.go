package cmd

import (
	"context"
	"encoding/json"
	"os"

	// Packages
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// EventCommands are the commands which watch the registry's events
type EventCommands struct {
	Events EventsCmd `cmd:"" name:"events" help:"Watch the registry's events, which are the changes to its providers, until interrupted." group:"LLM EVENTS"`
}

type EventsCmd struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints each event as a line of JSON, until the command is interrupted
func (cmd *EventsCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.EventsCmd", func(ctx context.Context, client *httpclient.Client) error {
		encoder := json.NewEncoder(os.Stdout)
		return client.WatchEvents(ctx, func(event providerschema.Event) error {
			return encoder.Encode(event)
		})
	})
}
