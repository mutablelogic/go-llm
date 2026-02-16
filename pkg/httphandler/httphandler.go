package httphandler

import (
	"errors"
	"net/http"

	// Package
	llm "github.com/mutablelogic/go-llm"
	manager "github.com/mutablelogic/go-llm/pkg/manager"
	server "github.com/mutablelogic/go-server"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// PathSeparator is used to split provider/model in URL path values
	PathSeparator = "/"
)

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
	register(AskHandler(manager))
	register(ChatHandler(manager))

	// Return any errors
	return result
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// httpErr converts an llm.Err to an httpresponse.Err, preserving the
// original error message. Unknown error codes map to 500.
func httpErr(err error) error {
	var llmErr llm.Err
	if !errors.As(err, &llmErr) {
		return err
	}
	switch llmErr {
	case llm.ErrNotFound:
		return httpresponse.ErrNotFound.With(err)
	case llm.ErrBadParameter:
		return httpresponse.ErrBadRequest.With(err)
	case llm.ErrConflict:
		return httpresponse.ErrConflict.With(err)
	case llm.ErrNotImplemented:
		return httpresponse.ErrNotImplemented.With(err)
	case llm.ErrInternalServerError:
		return httpresponse.ErrInternalError.With(err)
	default:
		return httpresponse.ErrInternalError.With(err)
	}
}
