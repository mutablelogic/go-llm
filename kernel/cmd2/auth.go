package cmd

import (
	// Packages
	httpclient "github.com/mutablelogic/go-auth/auth/httpclient"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// WithAuth returns auth client configured from the global HTTP flags.
func WithAuth(ctx server.Cmd, fn func(*httpclient.Client, string) error) error {
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
