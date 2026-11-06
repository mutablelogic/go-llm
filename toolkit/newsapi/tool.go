package newsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	if req.Query != "" {
		toolkit.Logf(ctx, "Searching the news for %q", req.Query)
	} else {
		toolkit.Logf(ctx, "Searching the news")
	}
	return a.client.Articles(ctx, &req)
}

////////////////////////////////////////////////////////////////////////////////
// HEADLINES

func (*headlines) Name() string {
	return "headlines"
}

func (*headlines) Description() string {
	return "Get breaking news headlines for a country, category, language, or search query, which are in English, unless one of them is given."
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

	// NewsAPI needs one of the sources, a query, a language, a country or a
	// category, so the headlines are in English, when there's none
	if req.Sources == "" && req.Query == "" && req.Language == "" && req.Country == "" && req.Category == "" {
		req.Language = "en"
	}

	toolkit.Logf(ctx, "%s", describeHeadlines(req))
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

	if req.Category != "" {
		toolkit.Logf(ctx, "Looking up sources of %s news", req.Category)
	} else {
		toolkit.Logf(ctx, "Looking up news sources")
	}
	return s.client.Sources(ctx, &req)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// describeHeadlines returns what the headlines are, in plain English, such
// as "Getting the latest technology headlines in GB about \"AI\""
func describeHeadlines(req httpclient.HeadlinesRequest) string {
	text := "Getting the latest"
	if req.Category != "" {
		text += " " + req.Category
	}
	text += " headlines"
	if req.Sources != "" {
		text += " from " + req.Sources
	} else if req.Country != "" {
		text += " in " + strings.ToUpper(req.Country)
	}
	if req.Query != "" {
		text += fmt.Sprintf(" about %q", req.Query)
	}
	return text
}

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
