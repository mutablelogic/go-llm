package cmd

import (
	"fmt"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// WithClient returns auth client configured from the global HTTP flags.
func WithClient(ctx server.Cmd, fn func(*httpclient.Client, string) error) error {
	return withClient(ctx, true, fn)
}

/*func withUnauthenticatedClient(ctx server.Cmd, fn func(*httpclient.Client, string) error) error {
	return withClient(ctx, false, fn)
}
*/

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

func withClient(ctx server.Cmd, authenticated bool, fn func(*httpclient.Client, string) error) error {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return err
	}
	if authenticated && len(opts) == 0 {
		return fmt.Errorf("authentication is required for this operation")
	}
	client, err := httpclient.New(endpoint, opts...)
	if err != nil {
		return err
	}
	return fn(client, endpoint)
}
