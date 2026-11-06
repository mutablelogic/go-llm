// Package httphandler has the registry's HTTP API
package httphandler

import (
	_ "embed"
	"errors"

	// Packages
	taggroup "github.com/mutablelogic/go-llm/internal/taggroup"
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

// RegisterHandlers registers the registry's API with the router: the
// sessions', and the agents', only when it has a store of them, as they'd
// be unavailable
func RegisterHandlers(r *registry.Registry, router *httprouter.Router) error {
	var documentation = openapi.ParseMarkdown(readme)

	// The tags, in order, of the registry's API, and of the toolkit's and
	// the broadcaster's, which are in the same group
	tags := []string{"Providers", "Models"}
	if r.SessionStore() != nil || r.EventStore() != nil {
		tags = append(tags, "Sessions")
	}
	tags = append(tags, "Inference")
	if r.Toolkit() != nil || r.AgentStore() != nil {
		tags = append(tags, taggroup.ToolsAndAgents)
	}
	tags = append(tags, "Events")
	taggroup.Add(router.Spec(), taggroup.LLM, tags...)

	// The handlers
	handlers := []error{
		providerHandlers(r, router, documentation),
		modelHandlers(r, router, documentation),
		embedHandlers(r, router, documentation),
		askHandlers(r, router, documentation),
		recognizeHandlers(r, router, documentation),
		speakHandlers(r, router, documentation),
		transcribeHandlers(r, router, documentation),
	}
	if r.SessionStore() != nil {
		handlers = append(handlers, sessionHandlers(r, router, documentation))
	}
	if r.AgentStore() != nil {
		handlers = append(handlers, agentHandlers(r, router, documentation))
	}
	if r.EventStore() != nil {
		handlers = append(handlers, eventHandlers(r, router, documentation))
	}
	return errors.Join(handlers...)
}
