package deepseek

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/deepseek/httpclient"
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

const (
	// defaultCount is the number of search results when it isn't set, and
	// maxCount the most, which DeepSeek allows more than, as each is a
	// summary of a page, of about 3,000 characters, and a model reads them
	// all
	defaultCount = 3
	maxCount     = 10

	// maxQuery is the most characters of a query, and of the person's
	// question, which DeepSeek accepts
	maxQuery = 128

	// maxURL is the longest URL which DeepSeek fetches
	maxURL = 8192
)

////////////////////////////////////////////////////////////////////////////////
// WEB SEARCH

func (*webSearch) Name() string {
	return "search"
}

func (*webSearch) Description() string {
	return "Search the web, and return the title, URL, site, date and a summary of each matching page. Each search is billed by DeepSeek."
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
	} else if utf8.RuneCountInString(req.Query) > maxQuery {
		return nil, llm.ErrBadParameter.Withf("query must be at most %d characters", maxQuery)
	}
	if req.Count == 0 {
		req.Count = defaultCount
	} else if req.Count < 0 || req.Count > maxCount {
		return nil, llm.ErrBadParameter.Withf("count must be between 1 and %d", maxCount)
	}
	if req.Question != nil {
		if question := strings.TrimSpace(*req.Question); question == "" {
			req.Question = nil
		} else if utf8.RuneCountInString(question) > maxQuery {
			return nil, llm.ErrBadParameter.Withf("ori_question must be at most %d characters", maxQuery)
		} else {
			req.Question = &question
		}
	}

	toolkit.Logf(ctx, "Searching the web for %q", req.Query)
	response, err := s.client.Search(ctx, &req)
	if err != nil {
		return nil, err
	}
	return response, nil
}

////////////////////////////////////////////////////////////////////////////////
// WEB FETCH

func (*webFetch) Name() string {
	return "fetch"
}

func (*webFetch) Description() string {
	return "Fetch a web page, and return its title, and its content as Markdown. Each fetch is billed by DeepSeek."
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

	// Validate the request: an http or https URL, with a host, and without
	// a username or password, which DeepSeek refuses
	if req.URL = strings.TrimSpace(req.URL); req.URL == "" {
		return nil, llm.ErrBadParameter.With("url is required")
	} else if len(req.URL) > maxURL {
		return nil, llm.ErrBadParameter.Withf("url must be at most %d characters", maxURL)
	} else if u, err := url.Parse(req.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return nil, llm.ErrBadParameter.Withf("url %q must be an http or https URL, with a host, and without a username or password", req.URL)
	}

	toolkit.Logf(ctx, "Reading %s", req.URL)
	response, err := f.client.Fetch(ctx, &req)
	if err != nil {
		return nil, err
	}
	return response, nil
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
