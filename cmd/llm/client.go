package main

import (
	"fmt"
	"net"
	"os"
	"strconv"

	// Packages
	client "github.com/mutablelogic/go-client"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Client returns an httpclient.Client configured from the global HTTP flags.
func (g *Globals) Client() (*httpclient.Client, error) {
	endpoint, err := g.clientEndpoint()
	if err != nil {
		return nil, err
	}
	return httpclient.New(endpoint, g.ClientOpts()...)
}

// ClientOpts returns the client options from global flags.
func (g *Globals) ClientOpts() []client.ClientOpt {
	opts := []client.ClientOpt{}
	if g.Debug || g.Verbose {
		opts = append(opts, client.OptTrace(os.Stderr, g.Verbose))
	}
	if g.tracer != nil {
		opts = append(opts, client.OptTracer(g.tracer))
	}
	if g.HTTP.Timeout > 0 {
		opts = append(opts, client.OptTimeout(g.HTTP.Timeout))
	}
	return opts
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// clientEndpoint returns the endpoint URL derived from the global HTTP flags.
func (g *Globals) clientEndpoint() (string, error) {
	scheme := "http"
	host, port, err := net.SplitHostPort(g.HTTP.Addr)
	if err != nil {
		return "", err
	}

	// Default host to localhost if empty (e.g., ":8084")
	if host == "" {
		host = "localhost"
	}

	// Parse port
	portn, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return "", err
	}
	if portn == 443 {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s:%v%s", scheme, host, portn, types.NormalisePath(g.HTTP.Prefix)), nil
}
