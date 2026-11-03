package httphandler

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
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
// TYPES

// deadlineWriter writes a stream to a client, with a deadline for each write,
// so a client which stops reading can't block it for longer than
// eventWriteTimeout, and is closed when a write fails
type deadlineWriter struct {
	http.ResponseWriter
	rc     *http.ResponseController
	once   sync.Once
	failed chan struct{}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func eventHandlers(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	router.Spec().AddTag("Events", documentation.Section(2, "Event Operations").Body)
	return router.Register("llm/event", nil, func(path httprequest.PathItem) {
		path.Tag("Events")

		// GET
		path.Get(eventWatchHandler(r), func(op httprequest.PathOperation) {
			op.Summary("Watch Events")
			op.Description(documentation.Section(3, "GET /llm/event").Body)
			op.TextStreamResponse(http.StatusOK)
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusNotAcceptable, "The response can't be any of the accepted types")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server is stopping")
		})
	})
}

// eventWatchHandler streams the registry's events, as server-sent events,
// until the client goes away, or the registry stops, as the server is
// stopping, when a shutdown event is sent before the stream ends
func eventWatchHandler(reg *registry.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Events are only streamed
		if accept, err := types.AcceptContentType(r); err != nil {
			httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			return
		} else if accept != "" && accept != types.ContentTypeAny && accept != types.ContentTypeTextStream {
			httpresponse.Error(w, httpresponse.Err(http.StatusNotAcceptable).Withf("cannot respond with %q", accept))
			return
		}

		// Watch the registry, until the client goes away, before the stream
		// starts, so a client which loads what it shows once the stream has
		// started misses no changes
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		ch := make(chan providerschema.Event, eventBuffer)

		// The server is stopping when the registry has stopped, so the
		// client isn't streamed to, and its connection is closed, so the
		// server isn't kept waiting for it
		done, err := reg.Watch(ctx, ch)
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
		dw := newDeadlineWriter(w)
		if err := dw.rc.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
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
			case <-dw.failed:
				// The client stopped reading, or went away
				return
			case err := <-done:
				// The registry stopped, as the server is stopping, so the
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

// newDeadlineWriter returns a writer of a stream to a client, with a deadline
// for each write
func newDeadlineWriter(w http.ResponseWriter) *deadlineWriter {
	return &deadlineWriter{ResponseWriter: w, rc: http.NewResponseController(w), failed: make(chan struct{})}
}

// Write writes to the client, by the deadline
func (w *deadlineWriter) Write(data []byte) (int, error) {
	w.deadline()
	n, err := w.ResponseWriter.Write(data)
	if err != nil {
		w.fail()
	}
	return n, err
}

// Flush sends what's written to the client, by the deadline
func (w *deadlineWriter) Flush() {
	w.deadline()
	if err := w.rc.Flush(); err != nil {
		w.fail()
	}
}

// Unwrap returns the response writer, for a response controller
func (w *deadlineWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// deadline sets the deadline of the next write
func (w *deadlineWriter) deadline() {
	if err := w.rc.SetWriteDeadline(time.Now().Add(eventWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		w.fail()
	}
}

// fail closes the channel which says a write failed
func (w *deadlineWriter) fail() {
	w.once.Do(func() { close(w.failed) })
}
