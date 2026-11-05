// Package httphandler has the heartbeats' HTTP API, which lists, creates,
// gets, changes and deletes them
package httphandler

import (
	_ "embed"
	"errors"
	"net/http"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	taggroup "github.com/mutablelogic/go-llm/internal/taggroup"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// heartbeatID is the path parameter of a heartbeat's identifier
type heartbeatID struct {
	ID string `json:"id" format:"uuid" help:"Identifier of the heartbeat"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed README.md
var readme []byte

// Tag is the tag of the heartbeats' API, in the "LLM" group
const Tag = "Heartbeats"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// RegisterHandlers registers the heartbeats' API with the router, at
// heartbeat, with the "Heartbeats" tag, in the "LLM" group
func RegisterHandlers(h *heartbeat.Heartbeats, router *httprouter.Router) error {
	documentation := openapi.ParseMarkdown(readme)
	router.Spec().AddTag(Tag, documentation.Section(2, "Heartbeat Operations").Body)
	taggroup.Add(router.Spec(), taggroup.LLM, Tag)
	return errors.Join(
		heartbeatsHandler(h, router, documentation),
		heartbeatHandler(h, router, documentation),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func heartbeatsHandler(h *heartbeat.Heartbeats, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("heartbeat", nil, func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.HeartbeatListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := h.List(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Heartbeats")
			op.Description(documentation.Section(3, "GET /heartbeat").Body)
			op.Query(jsonschema.MustFor[schema.HeartbeatListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.HeartbeatList](), "The heartbeats, the soonest first, and how many there are")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a session which isn't a UUID")
		})

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var meta schema.HeartbeatMeta
			if err := httprequest.Read(r, &meta); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if heartbeat, err := h.Create(r.Context(), meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), heartbeat)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Create Heartbeat")
			op.Description(documentation.Section(3, "POST /heartbeat").Body)
			op.RequestBody(jsonschema.MustFor[schema.HeartbeatMeta]())
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.Heartbeat](), "The heartbeat, which was created")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a heartbeat without a prompt, or a time which has passed")
		})
	})
}

func heartbeatHandler(h *heartbeat.Heartbeats, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("heartbeat/{id}", jsonschema.MustFor[heartbeatID](), func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			if id, err := parseID(r); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if heartbeat, err := h.Get(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), heartbeat)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Heartbeat")
			op.Description(documentation.Section(3, "GET /heartbeat/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Heartbeat](), "The heartbeat")
			op.ErrorResponse(http.StatusNotFound, "Heartbeat not found")
		})

		// PATCH
		path.Patch(func(w http.ResponseWriter, r *http.Request) {
			var req schema.HeartbeatRequest
			if id, err := parseID(r); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if heartbeat, err := h.Update(r.Context(), id, req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), heartbeat)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Update Heartbeat")
			op.Description(documentation.Section(3, "PATCH /heartbeat/{id}").Body)
			op.RequestBody(jsonschema.MustFor[schema.HeartbeatRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Heartbeat](), "The heartbeat, as it's changed")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusNotFound, "Heartbeat not found")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			if id, err := parseID(r); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if heartbeat, err := h.Delete(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), heartbeat)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Heartbeat")
			op.Description(documentation.Section(3, "DELETE /heartbeat/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Heartbeat](), "The heartbeat, as it was")
			op.ErrorResponse(http.StatusNotFound, "Heartbeat not found")
		})
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// parseID returns the identifier of the heartbeat in the request's path, or
// ErrBadParameter when it isn't a UUID
func parseID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil(), llm.ErrBadParameter.Withf("invalid heartbeat id %q", r.PathValue("id"))
	}
	return id, nil
}
