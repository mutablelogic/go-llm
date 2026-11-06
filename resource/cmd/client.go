// Package cmd has the resources' commands and flags
package cmd

import (
	"context"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/resource/httpclient"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ClientCommands are the commands which call the resources' API
type ClientCommands struct {
	ResourceCommands
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// withClient creates a client of the resources' API, from the command's
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
