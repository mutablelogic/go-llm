package ollama

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/ollama/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type webSearch struct {
	toolkit.Base
	client *httpclient.Client
}

type webFetch struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*webSearch)(nil)
var _ llm.Tool = (*webFetch)(nil)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultMaxResults is the number of search results when the maximum isn't set
const defaultMaxResults = 5

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewTools returns the Ollama web search and web fetch tools, which share a
// client with the API key
func NewTools(apiKey string, opts ...client.ClientOpt) ([]llm.Tool, error) {
	// Create a client
	client, err := httpclient.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}

	return []llm.Tool{
		&webSearch{client: client},
		&webFetch{client: client},
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// WEB SEARCH

func (*webSearch) Name() string {
	return "search"
}

func (*webSearch) Description() string {
	return "Search the web, and return the title, URL and a snippet of the content of each matching page."
}

// InputSchema returns the JSON schema for the tool input
func (*webSearch) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.SearchRequest]()
}

// Run the tool with the input
func (s *webSearch) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.SearchRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}

	// Validate the request
	if req.Query = strings.TrimSpace(req.Query); req.Query == "" {
		return nil, llm.ErrBadParameter.With("query is required")
	}
	if req.MaxResults == 0 {
		req.MaxResults = defaultMaxResults
	} else if req.MaxResults < 0 || req.MaxResults > 10 {
		return nil, llm.ErrBadParameter.With("max_results must be between 1 and 10")
	}

	return s.client.Search(ctx, &req)
}

////////////////////////////////////////////////////////////////////////////////
// WEB FETCH

func (*webFetch) Name() string {
	return "fetch"
}

func (*webFetch) Description() string {
	return "Fetch a web page, and return its title, main content and links."
}

// InputSchema returns the JSON schema for the tool input
func (*webFetch) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.FetchRequest]()
}

// Run the tool with the input
func (f *webFetch) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.FetchRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}

	// Validate the request
	if req.URL = strings.TrimSpace(req.URL); req.URL == "" {
		return nil, llm.ErrBadParameter.With("url is required")
	}

	return f.client.Fetch(ctx, &req)
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
