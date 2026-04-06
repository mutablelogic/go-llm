package httphandler

import (
	"context"
	"net/http"

	// Packages
	middleware "github.com/djthorpe/go-auth/pkg/middleware"
	llmmanager "github.com/mutablelogic/go-llm/pkg/manager"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func AskHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "ask", nil, httprequest.NewPathItem(
		"Ask operations",
		"Send a stateless prompt and get a response",
		"Responses",
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			_ = ask(r.Context(), manager, w, r)
		},
		"Ask model",
		opts.WithJSONRequest(jsonschema.MustFor[schema.AskRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.AskResponse]()),
		opts.WithTextStreamResponse(200, "SSE stream of assistant, thinking, tool, error, and result events."),
		opts.WithErrorResponse(400, "Invalid request body or ask failure."),
		opts.WithErrorResponse(404, "Model or provider not found."),
		opts.WithErrorResponse(409, "Multiple models matched; specify a provider."),
		opts.WithErrorResponse(406, "Unsupported Accept header."),
		opts.WithErrorResponse(501, "Provider does not support generation."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func ask(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.AskRequest
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	switch acceptType(r) {
	case acceptStream:
		stream := httpresponse.NewTextStream(w)
		if stream == nil {
			return httpresponse.Error(w, httpresponse.ErrInternalError)
		}
		defer stream.Close()

		fn := opt.StreamFn(func(role, text string) {
			switch role {
			case schema.RoleThinking:
				stream.Write(schema.EventThinking, schema.StreamDelta{Role: role, Text: text})
			case schema.RoleTool:
				stream.Write(schema.EventTool, schema.StreamDelta{Role: role, Text: text})
			default:
				stream.Write(schema.EventAssistant, schema.StreamDelta{Role: role, Text: text})
			}
		})

		resp, err := manager.Ask(ctx, req, middleware.UserFromContext(ctx), fn)
		if err != nil {
			stream.Write(schema.EventError, schema.StreamError{Error: err.Error()})
			return nil
		}

		stream.Write(schema.EventResult, resp)
		return nil
	case acceptJSON:
		resp, err := manager.Ask(ctx, req, middleware.UserFromContext(ctx), nil)
		if err != nil {
			return httpresponse.Error(w, schema.HTTPErr(err))
		}
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
	default:
		return httpresponse.Error(w, httpresponse.Err(http.StatusNotAcceptable))
	}
}
