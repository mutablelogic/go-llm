// Package cmd has the memory's commands and flags
package cmd

import (
	"context"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/memory/httpclient"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ClientCommands are the commands which call the memory's API
type ClientCommands struct {
	DetailCommands
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// withClient creates a client of the memory's API, from the command's
// endpoint and options, and calls the function with it, in a span with the
// name
func withClient(ctx server.Cmd, span string, fn func(context.Context, *httpclient.Client) error) (err error) {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return err
	}
	client, err := httpclient.New(endpoint, opts...)
	if err != nil {
		return err
	}
	parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), span)
	defer func() { endSpan(err) }()
	return fn(parent, client)
}
