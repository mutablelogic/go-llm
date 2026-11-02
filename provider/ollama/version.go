package ollama

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// versionResponse is the response from the version endpoint
type versionResponse struct {
	Version string `json:"version"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Version returns the Ollama server's version string.
func (c *Client) Version(ctx context.Context) (string, error) {
	var response versionResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("version")); err != nil {
		return "", err
	}
	return response.Version, nil
}
