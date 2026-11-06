// Package httphandler has the resources' HTTP API, which uploads, lists,
// gets, downloads and deletes them
package httphandler

import (
	_ "embed"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	taggroup "github.com/mutablelogic/go-llm/internal/taggroup"
	resource "github.com/mutablelogic/go-llm/resource"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// resourceHash is the path parameter of a resource's hash
type resourceHash struct {
	Hash string `json:"hash" help:"SHA-256 of the resource's content, in hexadecimal"`
}

// upload is the form of an upload, with its file, as the OpenAPI
// specification describes it
type upload struct {
	schema.ResourceCreateRequest
	File types.File `json:"file" help:"Content of the resource"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed README.md
var readme []byte

// Tag is the tag of the resources' API, in the "LLM" group
const Tag = "Resources"

// errNoContent is returned when an upload has no file
var errNoContent = llm.ErrBadParameter.With("missing content, which is the request body, or a file in a form")

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// RegisterHandlers registers the resources' API with the router, at
// resource, with the "Resources" tag, in the "LLM" group
func RegisterHandlers(manager *resource.Manager, router *httprouter.Router) error {
	documentation := openapi.ParseMarkdown(readme)
	router.Spec().AddTag(Tag, documentation.Section(2, "Resource Operations").Body)
	taggroup.Add(router.Spec(), taggroup.LLM, Tag)
	return errors.Join(
		resourcesHandler(manager, router, documentation),
		resourceHandler(manager, router, documentation),
		contentHandler(manager, router, documentation),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func resourcesHandler(manager *resource.Manager, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("resource", nil, func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.ResourceListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := manager.List(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Resources")
			op.Description(documentation.Section(3, "GET /resource").Body)
			op.Query(jsonschema.MustFor[schema.ResourceListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ResourceList](), "The resources, the most recently used first, and how many there are")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a session which isn't a UUID")
		})

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			// A large file can take longer to upload than the server's read
			// timeout
			_ = http.NewResponseController(w).SetReadDeadline(time.Time{})
			created, isNew, err := create(r, manager)
			if err != nil {
				httpresponse.Error(w, httpErr(err))
				return
			}
			status := http.StatusOK
			if isNew {
				status = http.StatusCreated
			}
			httpresponse.JSON(w, status, httprequest.Indent(r), created)
		}, func(op httprequest.PathOperation) {
			op.Summary("Upload Resource")
			op.Description(documentation.Section(3, "POST /resource").Body)
			op.Query(jsonschema.MustFor[schema.ResourceCreateRequest]())
			op.RequestBody(jsonschema.MustFor[upload](), types.ContentTypeFormData)
			op.RequestBody(nil, "*/*")
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.Resource](), "The resource, which was created")
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Resource](), "The resource with its content in its session, which is used again")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a session which isn't a UUID, or no content")
			op.ErrorResponse(http.StatusNotFound, "Session not found")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The content is larger than the most which is kept")
		})
	})
}

func resourceHandler(manager *resource.Manager, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("resource/{hash}", jsonschema.MustFor[resourceHash](), func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			if key, err := parseKey(r); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if got, err := manager.Get(r.Context(), key); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), key.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), got)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Resource")
			op.Description(documentation.Section(3, "GET /resource/{hash}").Body)
			op.Query(jsonschema.MustFor[schema.ResourceRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Resource](), "The resource")
			op.ErrorResponse(http.StatusBadRequest, "Invalid hash, or session")
			op.ErrorResponse(http.StatusNotFound, "Resource not found")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			if key, err := parseKey(r); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if deleted, err := manager.Delete(r.Context(), key); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), key.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), deleted)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Resource")
			op.Description(documentation.Section(3, "DELETE /resource/{hash}").Body)
			op.Query(jsonschema.MustFor[schema.ResourceRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Resource](), "The resource, as it was")
			op.ErrorResponse(http.StatusBadRequest, "Invalid hash, or session")
			op.ErrorResponse(http.StatusNotFound, "Resource not found")
		})
	})
}

func contentHandler(manager *resource.Manager, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("resource/{hash}/content", jsonschema.MustFor[resourceHash](), func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			key, err := parseKey(r)
			if err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
				return
			}
			got, content, err := manager.Open(r.Context(), key)
			if err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), key.String())
				return
			}
			defer content.Close()
			binary := httpresponse.Binary{
				ContentType: got.Type,
				Size:        types.Ptr(got.Size),
				ModTime:     got.Created,
				ETag:        got.Hash,
				Path:        got.Name,
				Disposition: httpresponse.DispositionInline,
			}
			_ = binary.Write(w, r, content)
		}, func(op httprequest.PathOperation) {
			op.Summary("Download Resource")
			op.Description(documentation.Section(3, "GET /resource/{hash}/content").Body)
			op.Query(jsonschema.MustFor[schema.ResourceRequest]())
			op.Response(http.StatusOK, "*/*", "The resource's content, with its type")
			op.ErrorResponse(http.StatusBadRequest, "Invalid hash, or session")
			op.ErrorResponse(http.StatusNotFound, "Resource not found")
		})
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// create creates the resource uploaded with the request, as its body, or
// the first file in its form, which is what the query's parameters, and
// the form's values before the file, say it is, and returns it, and whether
// it was created
func create(r *http.Request, manager *resource.Manager) (schema.Resource, bool, error) {
	var created schema.Resource
	var isNew, uploaded bool
	_, err := httprequest.Binary(r, func(_ string, values url.Values, file types.File) error {
		defer file.Body.Close()
		if uploaded {
			// Only the first file is the resource's content
			_, err := io.Copy(io.Discard, file.Body)
			return err
		}
		uploaded = true

		// What it is, from the query, and the form's values, which are
		// before the file, and the file
		query := r.URL.Query()
		for key, value := range values {
			query[key] = value
		}
		var req schema.ResourceCreateRequest
		if err := httprequest.Query(query, &req); err != nil {
			return llm.ErrBadParameter.With(err.Error())
		}
		meta, err := req.Meta()
		if err != nil {
			return err
		}
		if meta.Name == "" && file.Path != "" {
			meta.Name = path.Base(file.Path)
		}
		if meta.Type == "" {
			meta.Type = file.ContentType
		}
		created, isNew, err = manager.Create(r.Context(), meta, file.Body)
		return err
	})
	if err != nil {
		return schema.Resource{}, false, err
	} else if !uploaded {
		return schema.Resource{}, false, errNoContent
	}
	return created, isNew, nil
}

// httpErr returns the error as an HTTP error, which is 413 when the content
// was too large
func httpErr(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return httpresponse.Err(http.StatusRequestEntityTooLarge).Withf("the content is larger than %d bytes", tooLarge.Limit)
	}
	var httpErr httpresponse.Err
	if errors.As(err, &httpErr) {
		return err
	}
	return llm.HTTPErr(err)
}

// parseKey returns the key of the resource in the request: the hash in its
// path, and the session in its query, or none
func parseKey(r *http.Request) (schema.ResourceKey, error) {
	var req schema.ResourceRequest
	if err := httprequest.Query(r.URL.Query(), &req); err != nil {
		return schema.ResourceKey{}, llm.ErrBadParameter.With(err.Error())
	}
	return req.Key(r.PathValue("hash"))
}
