package httphandler

import (
	"errors"
	"net/http"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func askHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/ask", nil, func(path httprequest.PathItem) {
		path.Tag("Inference")

		// POST, asking a model for its reply to messages
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			// The body is limited, as messages can have images, which are
			// read into memory
			var req schema.AskRequest
			if err := limitBody(w, r, schema.AskBodyLimit); err != nil {
				httpresponse.Error(w, err)
			} else if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else {
				ask(w, r, req.Model, func(stream llm.GeneratorStreamFn) (*schema.AskResponse, error) {
					return reg.Ask(r.Context(), req, stream)
				})
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Ask")
			op.Description(documentation.Section(3, "POST /llm/ask").Body)
			op.RequestBody(jsonschema.MustFor[schema.AskRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.AskResponse](), "The model's reply, and the tokens used, or a stream of server-sent events when text/event-stream is accepted")
			op.TextStreamResponse(http.StatusOK)
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body, no model or messages, an option which is a field, an invalid schema, a tool named more than once, a message the provider can't send, or a request the provider rejects")
			op.ErrorResponse(http.StatusNotFound, "Model or tool not found")
			op.ErrorResponse(http.StatusNotAcceptable, "The response can't be any of the accepted types")
			op.ErrorResponse(http.StatusConflict, "More than one provider has a model with the name, or a tool's name without a namespace is ambiguous")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The request body is larger than 32 MiB")
			op.ErrorResponse(http.StatusNotImplemented, "The model's provider can't generate messages, or the model is known not to, or not to call tools, think, or see images, when it's asked to")
			op.ErrorResponse(http.StatusServiceUnavailable, "The model couldn't reply, or a provider which may have it can't be reached")
		})
	})
}

// ask writes the response of the function, which asks a model for its reply,
// as JSON. When the request accepts text/event-stream, the reply is streamed
// as server-sent events instead: an event for each piece of the reply as
// it's generated, named for what it is, text or thinking, and then a result
// event with the response, or an error event. The name is of the model, for
// errors. A model can take longer to reply than the server's write timeout,
// which is for the whole response, so it's cleared, and each write has its
// own deadline instead, as with the stream of events.
func ask(w http.ResponseWriter, r *http.Request, name string, fn func(llm.GeneratorStreamFn) (*schema.AskResponse, error)) {
	accept, err := types.AcceptContentType(r)
	if err != nil {
		httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
		return
	}
	dw := newDeadlineWriter(w)
	if err := dw.rc.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		httpresponse.Error(w, httpresponse.ErrInternalError.With(err.Error()))
		return
	}
	switch accept {
	case types.ContentTypeTextStream:
		stream := httpresponse.NewTextStream(dw, "X-Accel-Buffering", "no")
		if stream == nil {
			httpresponse.Error(w, httpresponse.ErrInternalError.With("streaming isn't supported"))
			return
		}
		defer stream.Close()
		if response, err := fn(func(what llm.StreamType, text string) {
			stream.Write(string(what), schema.StreamEvent{Text: text})
		}); err != nil {
			stream.Write(schema.EventError, schema.ErrorEvent{Error: err.Error()})
		} else {
			stream.Write(schema.EventResult, response)
		}
	case "", types.ContentTypeJSON, types.ContentTypeAny:
		if response, err := fn(nil); err != nil {
			httpresponse.Error(dw, llm.HTTPErr(err), name)
		} else {
			httpresponse.JSON(dw, http.StatusOK, httprequest.Indent(r), response)
		}
	default:
		httpresponse.Error(w, httpresponse.Err(http.StatusNotAcceptable).Withf("cannot respond with %q", accept))
	}
}
