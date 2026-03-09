package main

import (
	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// clientFor returns an httpclient.Client configured from the global HTTP flags.
func clientFor(ctx server.Cmd) (*httpclient.Client, error) {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return nil, err
	}
	return httpclient.New(endpoint, opts...)
}
