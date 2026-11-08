// Package httphandler has the HTTP API of the MCP connectors, which lists,
// gets, creates, updates and deletes them
package httphandler

import (
	_ "embed"
	"errors"

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

// RegisterHandlers registers the connectors' API with the router, at
// mcp/connector, with the "Connectors" tag, in the "LLM" group
func RegisterHandlers(r *registry.Registry, router *httprouter.Router) error {
	documentation := openapi.ParseMarkdown(readme)
	router.Spec().AddTag(Tag, documentation.Section(2, "Connector Operations").Body)
	taggroup.Add(router.Spec(), taggroup.LLM, Tag)
	return errors.Join(
		connectorCreateListHandlers(r, router, documentation),
		connectorHandlers(r, router, documentation),
		authorizeHandlers(r, router, documentation),
		templateHandlers(r, router, documentation),
	)
}
