package cmd

import (
	"context"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/toolkit/httpclient"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ClientCommands struct {
	ToolCommands
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// withToolkit creates an llm toolkit HTTP client from the command context
// and calls fn with it
func withToolkit(ctx server.Cmd, span string, fn func(context.Context, *httpclient.Client) error) (err error) {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return err
	} else if client, err := httpclient.New(endpoint, opts...); err != nil {
		return err
	} else {
		ctx, endfn := otel.StartSpan(ctx.Tracer(), ctx.Context(), span)
		defer func() { endfn(err) }()
		return fn(ctx, client)
	}
}
