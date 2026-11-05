// Package httphandler has the broadcaster's HTTP API, which streams its
// events to clients
package httphandler

import (
	_ "embed"

	// Packages
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed README.md
var readme []byte

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// RegisterHandlers registers the broadcaster's API with the router, which
// streams its events at llm/event, with the "Events" tag
func RegisterHandlers(b *broadcaster.Broadcaster, router *httprouter.Router) error {
	return eventHandlers(b, router, openapi.ParseMarkdown(readme))
}
