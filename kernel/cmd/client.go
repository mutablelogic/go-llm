package cmd

import (
	"errors"
	"fmt"
	"net/http"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/kernel/httpclient"
	server "github.com/mutablelogic/go-server"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
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
	if ctx.IsDebug() {
		return fn(client, endpoint)
	} else {
		return normalizeError(fn(client, endpoint))
	}
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}

	var response httpresponse.ErrResponse
	if errors.As(err, &response) {
		switch {
		case response.Reason != "":
			return fmt.Errorf("%s", response.Reason)
		case response.Code > 0:
			if text := http.StatusText(response.Code); text != "" {
				return fmt.Errorf("%s", text)
			}
		}
	}

	return err
}
