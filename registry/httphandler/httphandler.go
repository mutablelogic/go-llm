// Package httphandler has the registry's HTTP API
package httphandler

import (
	_ "embed"
	"errors"

	// Packages
	registry "github.com/mutablelogic/go-llm/registry"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed README.md
var readme []byte

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// RegisterHandlers registers the registry's API with the router
func RegisterHandlers(r *registry.Registry, router *httprouter.Router) error {
	var documentation = openapi.ParseMarkdown(readme)
	router.Spec().AddTagGroup("LLM", "Providers", "Models", "Sessions", "Inference", "Events")
	return errors.Join(
		providerHandlers(r, router, documentation),
		modelHandlers(r, router, documentation),
		sessionHandlers(r, router, documentation),
		embedHandlers(r, router, documentation),
		askHandlers(r, router, documentation),
		recognizeHandlers(r, router, documentation),
		transcribeHandlers(r, router, documentation),
	)
}
