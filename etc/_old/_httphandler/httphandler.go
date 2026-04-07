package httphandler

import (
	"errors"
	"net/http"

	// Package

	manager "github.com/mutablelogic/go-llm/kernel/manager"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	server "github.com/mutablelogic/go-server"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// PathSeparator is used to split provider/model in URL path values
	PathSeparator = "/"
)

// pathParamSchema is the JSON schema for string path/query parameters.
var pathParamSchema, _ = jsonschema.For[string]()

// queryUintSchema is the JSON schema for unsigned integer query parameters.
var queryUintSchema, _ = jsonschema.For[uint]()

// queryBoolSchema is the JSON schema for boolean query parameters.
var queryBoolSchema, _ = jsonschema.For[bool]()

// queryStringArraySchema is the JSON schema for string array query parameters.
var queryStringArraySchema, _ = jsonschema.For[[]string]()

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Router interface {
	RegisterFunc(path string, handler http.HandlerFunc, middleware bool, spec *openapi.PathItem) error
}

func RegisterHandlers(manager *manager.Manager, router server.HTTPRouter, middleware bool) error {
	var result error

	// Convenience function to register a handler and accumulate any errors
	register := func(path string, handler http.HandlerFunc, spec *openapi.PathItem) {
		result = errors.Join(result, router.(Router).RegisterFunc(path, handler, true, spec))
	}

	// Register handlers
	register(ModelListHandler(manager))
	register(ModelGetHandler(manager))
	register(ToolListHandler(manager))
	register(ToolGetHandler(manager))
	register(EmbeddingHandler(manager))
	register(SessionHandler(manager))
	register(SessionGetHandler(manager))
	register(AgentHandler(manager))
	register(AgentGetHandler(manager))
	register(AskHandler(manager))
	register(ChatHandler(manager))
	register(ConnectorListHandler(manager))
	register(ConnectorHandler(manager))

	// Add top-level tag descriptions for documentation tools (e.g. Swagger UI, Redoc)
	if spec := router.Spec(); spec != nil {
		spec.AddTag("Model", "Discover and inspect language models available across all configured providers.")
		spec.AddTag("Tool", "Discover and inspect tools that can be called by language models during inference.")
		spec.AddTag("Embedding", "Generate vector embeddings for text input using a configured embedding model.")
		spec.AddTag("Agent", "Create, update, delete and run agents — reusable prompt templates with model and tool bindings.")
		spec.AddTag("Session", "Manage conversation sessions, which maintain message history across multiple chat turns.")
		spec.AddTag("Chat", "Send messages to a language model, either statelessly (ask) or within a session (chat).")
		spec.AddTag("Connector", "Register and manage MCP server connectors that expose additional tools to language models.")
	}

	// Return any errors
	return result
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// httpErr converts an schema.Err to an httpresponse.Err, preserving the
// original error message. Unknown error codes map to 500.
func httpErr(err error) error {
	var llmErr schema.Err
	if !errors.As(err, &llmErr) {
		return err
	}
	switch llmErr {
	case schema.ErrNotFound:
		return httpresponse.ErrNotFound.With(err)
	case schema.ErrBadParameter:
		return httpresponse.ErrBadRequest.With(err)
	case schema.ErrConflict:
		return httpresponse.ErrConflict.With(err)
	case schema.ErrNotImplemented:
		return httpresponse.ErrNotImplemented.With(err)
	case schema.ErrInternalServerError:
		return httpresponse.ErrInternalError.With(err)
	default:
		return httpresponse.ErrInternalError.With(err)
	}
}
