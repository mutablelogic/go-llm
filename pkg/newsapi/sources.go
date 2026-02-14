package newsapi

import (
	"context"

	// Packages
	"github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Source struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Url         string `json:"url,omitempty"`
	Category    string `json:"category,omitempty"`
	Language    string `json:"language,omitempty"`
	Country     string `json:"country,omitempty"`
}

type respSources struct {
	Status  string   `json:"status"`
	Code    string   `json:"code,omitempty"`
	Message string   `json:"message,omitempty"`
	Sources []Source `json:"sources"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Sources returns all news sources based on the provided request parameters
func (c *Client) Sources(ctx context.Context, req *SourcesRequest) ([]Source, error) {
	var response respSources

	// Request -> Response
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("top-headlines/sources"), client.OptQuery(req.Values())); err != nil {
		return nil, err
	} else if response.Status != "ok" {
		return nil, llm.ErrBadParameter.Withf("%s: %s", response.Code, response.Message)
	}

	// Return success
	return response.Sources, nil
}
