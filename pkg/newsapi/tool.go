package newsapi

import (
	"context"
	"encoding/json"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	"github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type articles struct {
	client *Client
}

type headlines struct {
	client *Client
}

type sources struct {
	client *Client
}

var _ tool.Tool = (*articles)(nil)
var _ tool.Tool = (*headlines)(nil)
var _ tool.Tool = (*sources)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewTools(apikey string, opts ...client.ClientOpt) ([]tool.Tool, error) {
	// Create a client
	client, err := New(apikey, opts...)
	if err != nil {
		return nil, err
	}

	return []tool.Tool{
		&articles{client: client},
		&headlines{client: client},
		&sources{client: client},
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// ARTICLES

func (*articles) Name() string {
	return "newsapi_articles"
}

func (*articles) Description() string {
	return "Search for news articles given a query, date range, language, and other parameters."
}

// Return the JSON schema for the tool input
func (*articles) Schema() (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[ArticlesRequest](nil)
	if err != nil {
		return nil, err
	}

	// Add enum constraints for sortBy
	if sortBy, ok := schema.Properties["sortBy"]; ok && sortBy != nil {
		sortBy.Enum = []any{"relevancy", "popularity", "publishedAt"}
	}

	return schema, nil
}

// Run the tool with the given input
func (a *articles) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req ArticlesRequest

	// Unmarshal JSON input if provided
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}

	return a.client.Articles(ctx, &req)
}

///////////////////////////////////////////////////////////////////////////////
// HEADLINES

func (*headlines) Name() string {
	return "newsapi_headlines"
}

func (*headlines) Description() string {
	return "Get breaking news headlines for a country, category, or search query."
}

// Return the JSON schema for the tool input
func (*headlines) Schema() (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[HeadlinesRequest](nil)
	if err != nil {
		return nil, err
	}

	// Add enum constraints for category
	if category, ok := schema.Properties["category"]; ok && category != nil {
		category.Enum = []any{"business", "entertainment", "general", "health", "science", "sports", "technology"}
	}

	return schema, nil
}

// Run the tool with the given input
func (h *headlines) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req HeadlinesRequest

	// Unmarshal JSON input if provided
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}

	return h.client.Headlines(ctx, &req)
}

///////////////////////////////////////////////////////////////////////////////
// SOURCES

func (*sources) Name() string {
	return "newsapi_sources"
}

func (*sources) Description() string {
	return "Search for news sources given a category, language, country, and other parameters."
}

// Return the JSON schema for the tool input
func (*sources) Schema() (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[SourcesRequest](nil)
	if err != nil {
		return nil, err
	}

	// Add enum constraints for category
	if category, ok := schema.Properties["category"]; ok && category != nil {
		category.Enum = []any{"business", "entertainment", "general", "health", "science", "sports", "technology"}
	}

	return schema, nil
}

// Run the tool with the given input
func (s *sources) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req SourcesRequest

	// Unmarshal JSON input if provided
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}

	return s.client.Sources(ctx, &req)
}
