package httpclient

import (
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// REQUEST TYPES

// SearchRequest is a request to search the web
type SearchRequest struct {
	Query    string  `json:"query" help:"Search query, of at most 128 characters. It can use site:example.com and -site:example.com, lang:en, after:2025-01-01 and before:2025-12-31, -word, and \"exact phrase\"."`
	Count    int     `json:"count,omitempty" help:"Maximum number of results to return (1-10)" min:"1" max:"10" default:"3"`
	Question *string `json:"ori_question,omitempty" help:"The person's question, in their words, of at most 128 characters, which helps the search understand the query" optional:""`
}

// FetchRequest is a request to fetch a web page
type FetchRequest struct {
	URL string `json:"url" help:"URL of the web page to fetch"`
}

////////////////////////////////////////////////////////////////////////////////
// RESPONSE TYPES

// SearchResponse is the results of searching the web
type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

// SearchResult is a web page which matched the search query, with a summary
// of it for models, and when it was published, when that's known
type SearchResult struct {
	Title    string     `json:"title"`
	URL      string     `json:"url"`
	SiteName *string    `json:"site_name,omitempty"`
	Date     *time.Time `json:"date,omitempty"`
	Summary  string     `json:"summary"`
}

// FetchResponse is a fetched web page, whose content is Markdown
type FetchResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r SearchResponse) String() string {
	return types.Stringify(r)
}

func (r FetchResponse) String() string {
	return types.Stringify(r)
}
