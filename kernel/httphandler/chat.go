package httphandler

import (
	"context"
	"net/http"

	// Packages
	middleware "github.com/djthorpe/go-auth/pkg/middleware"
	llmmanager "github.com/mutablelogic/go-llm/kernel/manager"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func ChatHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "chat", nil, httprequest.NewPathItem(
		"Chat operations",
		"Send a message within an existing session and get a response",
		"Responses",
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			_ = chat(r.Context(), manager, w, r)
		},
		"Chat within session",
		opts.WithJSONRequest(jsonschema.MustFor[schema.ChatRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.ChatResponse]()),
		opts.WithTextStreamResponse(200, "SSE stream of assistant, thinking, tool, error, and result events."),
		opts.WithErrorResponse(400, "Invalid request body or chat failure."),
		opts.WithErrorResponse(404, "Session not found."),
		opts.WithErrorResponse(406, "Unsupported Accept header."),
		opts.WithErrorResponse(501, "Provider does not support generation."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func chat(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.ChatRequest
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

		resp, err := manager.Chat(ctx, req, fn, middleware.UserFromContext(ctx))
		if err != nil {
			stream.Write(schema.EventError, schema.StreamError{Error: err.Error()})
			return nil
		}

		stream.Write(schema.EventResult, resp)
		return nil
	case acceptJSON:
		resp, err := manager.Chat(ctx, req, nil, middleware.UserFromContext(ctx))
		if err != nil {
			return httpresponse.Error(w, schema.HTTPErr(err))
		}
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
	default:
		return httpresponse.Error(w, httpresponse.Err(http.StatusNotAcceptable))
	}
}
