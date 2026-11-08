// Package httphandler has the HTTP API of the MCP connectors, which lists,
// gets, creates, updates and deletes them
package httphandler

import (
	_ "embed"
	"errors"
	"net/url"

	// Packages
	taggroup "github.com/mutablelogic/go-llm/internal/taggroup"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed README.md
var readme []byte

// Tag is the tag of the connectors' API, in the "LLM" group
const Tag = "Connectors"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Opt is an option of the connectors' API
type Opt func(*opt)

type opt struct {
	url func() *url.URL // The server's own URL, as it says it is
}

// WithURL is the function which returns the server's own URL, as it says
// it is, such as https://llm.example.com/api, which a person's browser is
// redirected back to, once they've signed in to a connector's server, when
// it's https, as it's then the URL the server was given, such as the name
// of a proxy it's behind, rather than its listen address, and otherwise
// the request's origin is
func WithURL(fn func() *url.URL) Opt {
	return func(o *opt) {
		o.url = fn
	}
}

// RegisterHandlers registers the connectors' API with the router, at
// mcp/connector, with the "Connectors" tag, in the "LLM" group
func RegisterHandlers(r *registry.Registry, router *httprouter.Router, opts ...Opt) error {
	var o opt
	for _, fn := range opts {
		fn(&o)
	}
	documentation := openapi.ParseMarkdown(readme)
	router.Spec().AddTag(Tag, documentation.Section(2, "Connector Operations").Body)
	taggroup.Add(router.Spec(), taggroup.LLM, Tag)
	return errors.Join(
		connectorCreateListHandlers(r, router, documentation),
		connectorHandlers(r, router, documentation),
		authorizeHandlers(r, router, documentation, o.url),
		templateHandlers(r, router, documentation),
	)
}
