package newsapi

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/newsapi/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type articles struct {
	toolkit.Base
	client *httpclient.Client
}

type headlines struct {
	toolkit.Base
	client *httpclient.Client
}

type sources struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*articles)(nil)
var _ llm.Tool = (*headlines)(nil)
var _ llm.Tool = (*sources)(nil)

////////////////////////////////////////////////////////////////////////////////
// ARTICLES

func (*articles) Name() string {
	return "articles"
}

func (*articles) Description() string {
	return "Search for news articles given a query, date range, language, and other parameters."
}

// InputSchema returns the JSON schema for the tool input
func (*articles) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.ArticlesRequest]()
}

// Run the tool with the input
func (a *articles) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.ArticlesRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}

	return a.client.Articles(ctx, &req)
}

////////////////////////////////////////////////////////////////////////////////
// HEADLINES

func (*headlines) Name() string {
	return "headlines"
}

func (*headlines) Description() string {
	return "Get breaking news headlines for a country, category, or search query."
}

// InputSchema returns the JSON schema for the tool input
func (*headlines) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.HeadlinesRequest]()
}

// Run the tool with the input
func (h *headlines) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.HeadlinesRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}

	// Sources can't be mixed with a country or category
	if req.Sources != "" && (req.Country != "" || req.Category != "") {
		return nil, llm.ErrBadParameter.With("sources can't be mixed with country or category")
	}

	return h.client.Headlines(ctx, &req)
}

////////////////////////////////////////////////////////////////////////////////
// SOURCES

func (*sources) Name() string {
	return "sources"
}

func (*sources) Description() string {
	return "Search for news sources given a category, language, country, and other parameters."
}

// InputSchema returns the JSON schema for the tool input
func (*sources) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.SourcesRequest]()
}

// Run the tool with the input
func (s *sources) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.SourcesRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}

	return s.client.Sources(ctx, &req)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// unmarshal decodes the tool input, which may be empty, into v
func unmarshal(input json.RawMessage, v any) error {
	if len(input) == 0 {
		return nil
	}
	if err := json.Unmarshal(input, v); err != nil {
		return llm.ErrBadParameter.Withf("invalid input: %v", err)
	}
	return nil
}
