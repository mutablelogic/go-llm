package httphandler

import (
	"context"
	"errors"
	"net/http"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	schema "github.com/mutablelogic/go-llm/broadcaster/schema"
	deadline "github.com/mutablelogic/go-llm/internal/deadline"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// eventBuffer is how many events are kept for a client, while it reads the
// ones before them, before they're dropped
const eventBuffer = 16

// eventWriteTimeout is the longest a write to a client's stream can take,
// before the client is taken to have stopped reading
var eventWriteTimeout = 30 * time.Second

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func eventHandlers(b *broadcaster.Broadcaster, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	router.Spec().AddTag("Events", documentation.Section(2, "Event Operations").Body)
	return router.Register("events", nil, func(path httprequest.PathItem) {
		path.Tag("Events")

		// GET
		path.Get(eventWatchHandler(b), func(op httprequest.PathOperation) {
			op.Summary("Watch Events")
			op.Description(documentation.Section(3, "GET /events").Body)
			op.TextStreamResponse(http.StatusOK)
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusNotAcceptable, "The response can't be any of the accepted types")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server is stopping")
		})
	})
}

// eventWatchHandler streams the broadcaster's events, as server-sent events,
// until the client goes away, or the broadcaster stops, as the server is
// stopping, when a shutdown event is sent before the stream ends
func eventWatchHandler(b *broadcaster.Broadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Events are only streamed
		if accept, err := types.AcceptContentType(r); err != nil {
			httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			return
		} else if accept != "" && accept != types.ContentTypeAny && accept != types.ContentTypeTextStream {
			httpresponse.Error(w, httpresponse.Err(http.StatusNotAcceptable).Withf("cannot respond with %q", accept))
			return
		}

		// Watch the broadcaster, until the client goes away, before the stream
		// starts, so a client which loads what it shows once the stream has
		// started misses no changes
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		ch := make(chan schema.Event, eventBuffer)

		// The server is stopping when the broadcaster has stopped, so the
		// client isn't streamed to, and its connection is closed, so the
		// server isn't kept waiting for it
		done, err := b.Watch(ctx, ch)
		if err != nil {
			if errors.Is(err, llm.ErrServiceUnavailable) {
				w.Header().Set("Connection", "close")
			}
			httpresponse.Error(w, llm.HTTPErr(err))
			return
		}

		// The stream lasts as long as the client watches, so it isn't cut off
		// by the server's write timeout, which is for the whole response, but
		// each write has its own deadline. A proxy, such as nginx, doesn't
		// buffer it. A writer which can't have its deadline set is streamed
		// to until the server's timeout.
		dw, err := deadline.New(w, eventWriteTimeout)
		if err != nil {
			httpresponse.Error(w, httpresponse.ErrInternalError.With(err.Error()))
			return
		}

		// Stream the events
		stream := httpresponse.NewTextStream(dw, "X-Accel-Buffering", "no")
		if stream == nil {
			httpresponse.Error(w, httpresponse.ErrInternalError.With("streaming isn't supported"))
			return
		}
		defer stream.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case <-dw.Failed():
				// The client stopped reading, or went away
				return
			case err := <-done:
				// The broadcaster stopped, as the server is stopping, so the
				// client is told, and the stream ends, so the server isn't
				// waiting for it. The event has data, as an event without
				// any isn't dispatched by a browser.
				if err != nil {
					stream.Write(schema.EventShutdown, struct{}{})
				}
				return
			case event := <-ch:
				stream.Write(schema.EventName, event)
			}
		}
	}
}
