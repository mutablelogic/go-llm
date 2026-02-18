package httpclient

import (
	"context"
	"fmt"
	"net/url"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// GetCredential retrieves the credential stored for the given server URL.
func (c *Client) GetCredential(ctx context.Context, url_ string) (*schema.OAuthCredentials, error) {
	if url_ == "" {
		return nil, fmt.Errorf("server URL cannot be empty")
	}

	// Create request
	req := client.NewRequest()
	reqOpts := []client.RequestOpt{client.OptPath("credential", url.PathEscape(url_))}

	// Perform request
	var response schema.OAuthCredentials
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// SetCredential stores (or updates) the credential for the given server URL.
func (c *Client) SetCredential(ctx context.Context, url_ string, cred schema.OAuthCredentials) error {
	if url_ == "" {
		return fmt.Errorf("server URL cannot be empty")
	}

	// Create request
	req, err := client.NewJSONRequest(cred)
	if err != nil {
		return err
	}
	reqOpts := []client.RequestOpt{client.OptPath("credential", url.PathEscape(url_))}

	// Perform request
	if err := c.DoWithContext(ctx, req, nil, reqOpts...); err != nil {
		return err
	}

	// Return success
	return nil
}

// DeleteCredential removes the credential for the given server URL.
func (c *Client) DeleteCredential(ctx context.Context, url_ string) error {
	if url_ == "" {
		return fmt.Errorf("server URL cannot be empty")
	}

	// Create request
	reqOpts := []client.RequestOpt{client.OptPath("credential", url.PathEscape(url_))}

	// Perform request
	if err := c.DoWithContext(ctx, client.MethodDelete, nil, reqOpts...); err != nil {
		return err
	}

	// Return success
	return nil
}
