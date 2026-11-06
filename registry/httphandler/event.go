package httphandler

import (
	"errors"
	"net/http"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// eventID is the path parameter of an event's identifier
type eventID struct {
	ID uuid.UUID `json:"id" help:"Event identifier"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// eventHandlers registers the handlers of the outcomes of events, which are
// in the Sessions tag, as they're of sessions
func eventHandlers(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return errors.Join(
		eventListHandlers(r, router, documentation),
		eventGetDeleteHandlers(r, router, documentation),
	)
}

func eventListHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/event", nil, func(path httprequest.PathItem) {
		path.Tag("Sessions")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.EventListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := reg.ListEvents(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Events")
			op.Description(documentation.Section(3, "GET /llm/event").Body)
			op.Query(jsonschema.MustFor[schema.EventListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.EventList](), "The outcomes of events, the most recent first")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a session which isn't a UUID, or a status which isn't one")
		})
	})
}

func eventGetDeleteHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/event/{id}", jsonschema.MustFor[eventID](), func(path httprequest.PathItem) {
		path.Tag("Sessions")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			if id, err := uuid.Parse(r.PathValue("id")); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("invalid event %q", r.PathValue("id")))
			} else if event, err := reg.GetEvent(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), event)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Event")
			op.Description(documentation.Section(3, "GET /llm/event/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Event](), "The event's outcome")
			op.ErrorResponse(http.StatusNotFound, "Event not found")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			if id, err := uuid.Parse(r.PathValue("id")); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("invalid event %q", r.PathValue("id")))
			} else if event, err := reg.DeleteEvent(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), event)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Event")
			op.Description(documentation.Section(3, "DELETE /llm/event/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Event](), "The event's outcome, as it was")
			op.ErrorResponse(http.StatusNotFound, "Event not found")
			op.ErrorResponse(http.StatusConflict, "The event was changed as it was deleted")
		})
	})
}
