package httphandler

import (
	"errors"
	"net/http"

	// Packages
	llm "github.com/mutablelogic/go-llm"
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

// memoryKey is the path parameter of a memory's key
type memoryKey struct {
	Key string `json:"key" help:"Key of the memory, such as kitchen_lights"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func memoryHandlers(m *memory.Memory, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return errors.Join(
		memoriesHandler(m, router, documentation),
		memoryHandler(m, router, documentation),
	)
}

func memoriesHandler(m *memory.Memory, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("memory/memory", nil, func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.MemoryListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := m.ListMemories(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Memories")
			op.Description(documentation.Section(3, "GET /memory/memory").Body)
			op.Query(jsonschema.MustFor[schema.MemoryListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.MemoryList](), "The memories, the most recently changed first, and how many there are")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
		})
	})
}

func memoryHandler(m *memory.Memory, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("memory/memory/{key}", jsonschema.MustFor[memoryKey](), func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			key := r.PathValue("key")
			if memory, err := m.GetMemory(r.Context(), key); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), key)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), memory)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Memory")
			op.Description(documentation.Section(3, "GET /memory/memory/{key}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Memory](), "The memory")
			op.ErrorResponse(http.StatusNotFound, "Memory not found")
		})

		// PUT
		path.Put(func(w http.ResponseWriter, r *http.Request) {
			var req schema.MemoryRequest
			key := r.PathValue("key")
			if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if memory, created, err := m.Remember(r.Context(), schema.MemoryMeta{Key: key, Description: req.Description, Body: req.Body}, req.Version); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), key)
			} else if created {
				httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), memory)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), memory)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Remember Memory")
			op.Description(documentation.Section(3, "PUT /memory/memory/{key}").Body)
			op.RequestBody(jsonschema.MustFor[schema.MemoryRequest]())
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.Memory](), "The memory, which was created")
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Memory](), "The memory, which replaced the memory at the version, when it's changed")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a key which isn't an identifier, or a memory without a description, or body")
			op.ErrorResponse(http.StatusConflict, "The version isn't the memory's version, or there's a memory with the key, and no version")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			var req schema.MemoryForgetRequest
			key := r.PathValue("key")
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
				return
			}
			var result schema.Memory
			var err error
			if req.Line != "" {
				result, err = m.ForgetLine(r.Context(), key, req.Line)
			} else {
				result, err = m.Forget(r.Context(), key)
			}
			if err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), key)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), result)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Forget Memory")
			op.Description(documentation.Section(3, "DELETE /memory/memory/{key}").Body)
			op.Query(jsonschema.MustFor[schema.MemoryForgetRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Memory](), "The memory, as it is, without the line, or as it was, when it's forgotten")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusNotFound, "The memory isn't found, or hasn't the line")
		})
	})
}
