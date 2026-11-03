package httphandler

import (
	_ "embed"
	"errors"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed README.md
var readme []byte

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// RegisterHandlers registers the handlers for the toolkit's tools, with the
// "Tools" tag
func RegisterHandlers(tools *toolkit.Toolkit, router *httprouter.Router) error {
	var documentation = openapi.ParseMarkdown(readme)
	return errors.Join(
		toolHandlers(tools, router, documentation),
	)
}
