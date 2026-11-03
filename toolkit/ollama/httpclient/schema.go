package httpclient

import (
	"encoding/json"
	"io"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// REQUEST TYPES

// SearchRequest is a request to search the web
type SearchRequest struct {
	Query      string `json:"query" help:"Search query"`
	MaxResults int    `json:"max_results,omitempty" help:"Maximum number of results to return (1-10)" min:"1" max:"10" default:"5"`
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

// SearchResult is a web page which matched the search query
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"` // A snippet of the page's content
}

// FetchResponse is a fetched web page
type FetchResponse struct {
	Title   string   `json:"title"`
	Content string   `json:"content"` // The page's main content
	Links   []string `json:"links,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r SearchResponse) String() string {
	return types.Stringify(r)
}

func (r FetchResponse) String() string {
	return types.Stringify(r)
}

////////////////////////////////////////////////////////////////////////////////
// UNMARSHAL

// Ollama returns its web search and web fetch responses as JSON, but with
// the content type "text/html", so they're decoded as JSON whatever their
// content type

var _ client.Unmarshaler = (*SearchResponse)(nil)
var _ client.Unmarshaler = (*FetchResponse)(nil)

func (r *SearchResponse) Unmarshal(_ http.Header, body io.Reader) error {
	return json.NewDecoder(body).Decode(r)
}

func (r *FetchResponse) Unmarshal(_ http.Header, body io.Reader) error {
	return json.NewDecoder(body).Decode(r)
}
