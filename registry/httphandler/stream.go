package httphandler

import (
	"net/http"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	deadline "github.com/mutablelogic/go-llm/internal/deadline"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// streamWriteFn writes an event, with its name, and its data, as JSON, to a
// stream of server-sent events
type streamWriteFn func(event string, data any)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// streamWriteTimeout is the longest a write to a client's stream can take,
// before the client is taken to have stopped reading
var streamWriteTimeout = 30 * time.Second

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// respond writes the response of the function, which asks a model for
// something, as JSON, when the function is called with a nil write
// function. When the request accepts text/event-stream, the function is
// called with a function which writes events, as server-sent events, such
// as for each piece of a model's reply as it's generated, and then a result
// event, with the response, or an error event, whose message is as it would
// be without a stream, such as "Not Found: model ...". The name is of the model,
// for errors. A model can take longer than the server's write timeout,
// which is for the whole response, so it's cleared, and each write has its
// own deadline instead, as with the stream of events.
func respond[T any](w http.ResponseWriter, r *http.Request, name string, fn func(streamWriteFn) (T, error)) {
	accept, err := types.AcceptContentType(r)
	if err != nil {
		httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
		return
	}
	dw, err := deadline.New(w, streamWriteTimeout)
	if err != nil {
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
		if response, err := fn(func(event string, data any) {
			stream.Write(event, data)
		}); err != nil {
			stream.Write(schema.EventError, schema.ErrorEvent{Error: llm.HTTPErr(err).Error()})
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
