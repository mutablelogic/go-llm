package cmd

import (

	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// WithClient returns auth client configured from the global HTTP flags.
func WithClient(ctx server.Cmd, fn func(*httpclient.Client, string) error) error {
	return withClient(ctx, fn)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

func withClient(ctx server.Cmd, fn func(*httpclient.Client, string) error) error {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return err
	}
	client, err := httpclient.New(endpoint, opts...)
	if err != nil {
		return err
	}
	return fn(client, endpoint)
}
