// Package httphandler has the memory's HTTP API, which gets, sets and
// forgets the details it keeps, and lists, gets, remembers and forgets its
// memories
package httphandler

import (
	_ "embed"
	"errors"
	"net/http"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	taggroup "github.com/mutablelogic/go-llm/internal/taggroup"
	memory "github.com/mutablelogic/go-llm/memory"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// detailField is the path parameter of a detail's field
type detailField struct {
	Field string `json:"field" enum:"name,assistant,location,timezone,language" help:"Field of the detail"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed README.md
var readme []byte

// Tag is the tag of the memory's API, in the "LLM" group
const Tag = "Memory"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// RegisterHandlers registers the memory's API with the router, at
// memory/detail, and memory/memory, with the "Memory" tag, in the "LLM"
// group
func RegisterHandlers(m *memory.Memory, router *httprouter.Router) error {
	documentation := openapi.ParseMarkdown(readme)
	router.Spec().AddTag(Tag, documentation.Section(2, "Detail Operations").Body+"\n\n"+documentation.Section(2, "Memory Operations").Body)
	taggroup.Add(router.Spec(), taggroup.LLM, Tag)
	return errors.Join(
		detailsHandlers(m, router, documentation),
		detailHandlers(m, router, documentation),
		memoryHandlers(m, router, documentation),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func detailsHandlers(m *memory.Memory, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("memory/detail", nil, func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), m.Details())
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Details")
			op.Description(documentation.Section(3, "GET /memory/detail").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Details](), "The details which are known, or declined")
		})
	})
}

func detailHandlers(m *memory.Memory, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("memory/detail/{field}", jsonschema.MustFor[detailField](), func(path httprequest.PathItem) {
		path.Tag(Tag)

		// PUT
		path.Put(func(w http.ResponseWriter, r *http.Request) {
			var detail schema.Detail
			if field, err := schema.ParseField(r.PathValue("field")); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if err := httprequest.Read(r, &detail); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if details, err := m.SetDetail(r.Context(), field, &detail); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), string(field))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), details)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Set Detail")
			op.Description(documentation.Section(3, "PUT /memory/detail/{field}").Body)
			op.RequestBody(jsonschema.MustFor[schema.Detail]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Details](), "The details, with the detail set")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a field which isn't one of the fields, or a detail which isn't valid")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			if field, err := schema.ParseField(r.PathValue("field")); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if details, err := m.SetDetail(r.Context(), field, nil); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), string(field))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), details)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Forget Detail")
			op.Description(documentation.Section(3, "DELETE /memory/detail/{field}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Details](), "The details, without the detail")
			op.ErrorResponse(http.StatusBadRequest, "The field isn't one of the fields")
		})
	})
}
