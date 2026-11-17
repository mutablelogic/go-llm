package httphandler

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	// Packages
	"errors"
	llm "github.com/mutablelogic/go-llm"
	taggroup "github.com/mutablelogic/go-llm/internal/taggroup"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// workspaceName is the path parameter naming a workspace
type workspaceName struct {
	Name string `json:"name" help:"Name of the workspace, which is its namespace's"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed README.md
var readme []byte

// MaxHashed is the largest content which is read, and hashed, for its
// entity tag, before it's written. Larger content is written as it's read,
// without one, in ranges, when it can seek.
const MaxHashed = 8 * 1024 * 1024

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// RegisterHandlers registers the handlers of the workspaces, and their
// resources, with the "Workspaces" tag, in the "LLM" group
func RegisterHandlers(workspaces *workspace.Registry, router *httprouter.Router) error {
	documentation := openapi.ParseMarkdown(readme)
	router.Spec().AddTag(taggroup.Workspaces, documentation.Section(2, "Workspace Operations").Body)
	taggroup.Add(router.Spec(), taggroup.LLM, taggroup.Workspaces)
	return errors.Join(
		workspaceListHandlers(workspaces, router, documentation),
		workspaceGetHandlers(workspaces, router, documentation),
		resourceListHandlers(workspaces, router, documentation),
		contentHandlers(workspaces, router, documentation),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func workspaceListHandlers(workspaces *workspace.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/workspace", nil, func(path httprequest.PathItem) {
		path.Tag(taggroup.Workspaces)
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), workspaces.ListWorkspaces(r.Context()))
		}, func(op httprequest.PathOperation) {
			op.Summary("List Workspaces")
			op.Description(documentation.Section(3, "GET /llm/workspace").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.WorkspaceList](), "List of workspaces")
		})
	})
}

func workspaceGetHandlers(workspaces *workspace.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/workspace/{name}", jsonschema.MustFor[workspaceName](), func(path httprequest.PathItem) {
		path.Tag(taggroup.Workspaces)
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if ws, err := workspaces.GetWorkspace(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), ws)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Workspace")
			op.Description(documentation.Section(3, "GET /llm/workspace/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Workspace](), "Workspace")
			op.ErrorResponse(http.StatusNotFound, "Workspace not found")
		})
	})
}

func resourceListHandlers(workspaces *workspace.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/workspace/{name}/resource", jsonschema.MustFor[workspaceName](), func(path httprequest.PathItem) {
		path.Tag(taggroup.Workspaces)
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.ListRequest
			name := r.PathValue("name")
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := workspaces.ListResources(r.Context(), name, req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Resources")
			op.Description(documentation.Section(3, "GET /llm/workspace/{name}/resource").Body)
			op.Query(jsonschema.MustFor[schema.ListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ListResponse](), "A page of the workspace's folders and resources")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusNotFound, "Workspace not found")
		})
	})
}

func contentHandlers(workspaces *workspace.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/workspace/{name}/content", jsonschema.MustFor[workspaceName](), func(path httprequest.PathItem) {
		path.Tag(taggroup.Workspaces)
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.ReadRequest
			name := r.PathValue("name")
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if binary, content, err := read(r.Context(), workspaces, name, req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.URI)
			} else {
				// The content is written as it's read, so an error is
				// responded with, unless it's being written
				writing := false
				if err := content(r.Context(), func(body io.Reader) error {
					writing = true
					return binary.Write(w, r, body)
				}); err != nil && !writing {
					httpresponse.Error(w, llm.HTTPErr(err), req.URI)
				}
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Read Resource")
			op.Description(documentation.Section(3, "GET /llm/workspace/{name}/content").Body)
			op.Query(jsonschema.MustFor[schema.ReadRequest]())
			op.Response(http.StatusOK, "*/*", "The resource's content, with its type, or, with markdown, a Markdown document")
			op.Response(http.StatusPartialContent, "*/*", "The range of the resource's content which was asked for")
			op.Response(http.StatusNotModified, "", "Not modified, as the request's If-None-Match, or If-Modified-Since, says")
			op.ErrorResponse(http.StatusPreconditionFailed, "The request's If-Match, or If-Unmodified-Since, fails")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a format which isn't raw or markdown")
			op.ErrorResponse(http.StatusNotFound, "Workspace, or resource, not found")
			op.ErrorResponse(http.StatusNotImplemented, "The workspace doesn't read its resources, or, with markdown, the resource has no Markdown")
		})
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// read reads the workspace's resource, as it is, or as a Markdown document,
// and returns how it's written, and its content. Its entity tag is the
// SHA-256 of its content, as a workspace's resource has none of its own, so
// a request for it, again, which has it, isn't sent it, unless it's larger
// than MaxHashed, when it's read as it's written, without one.
func read(ctx context.Context, workspaces *workspace.Registry, name string, req schema.ReadRequest) (httpresponse.Binary, llm.ReadFunc, error) {
	if strings.TrimSpace(req.URI) == "" {
		return httpresponse.Binary{}, nil, llm.ErrBadParameter.With("missing uri")
	}
	resource, err := workspaces.Read(ctx, name, req.URI)
	if err != nil {
		return httpresponse.Binary{}, nil, err
	}
	binary := httpresponse.Binary{Path: resource.Name(), Disposition: httpresponse.DispositionInline}
	if req.Download {
		binary.Disposition = httpresponse.DispositionAttachment
	}
	var data []byte
	switch strings.ToLower(strings.TrimSpace(req.Format)) {
	case "", "raw":
		binary.ContentType = resource.Type()
		if size, ok := llm.ContentLength(resource); ok && size > MaxHashed {
			content, err := resource.Read(ctx)
			if err != nil {
				return httpresponse.Binary{}, nil, err
			}
			binary.Size = types.Ptr(size)
			return binary, content, nil
		}
		if data, err = llm.ReadAll(ctx, resource); err != nil {
			return httpresponse.Binary{}, nil, err
		}
	case "markdown":
		part, err := workspace.ReadMarkdown(ctx, resource, req.Offset, req.Limit)
		if err != nil {
			return httpresponse.Binary{}, nil, err
		}
		data, binary.ContentType = []byte(part.Document()), "text/markdown; charset=utf-8"
		if binary.Path != "" {
			binary.Path += ".md"
		}
	default:
		return httpresponse.Binary{}, nil, llm.ErrBadParameter.Withf("unknown format %q, which isn't raw or markdown", req.Format)
	}
	sum := sha256.Sum256(data)
	binary.ETag, binary.Size = hex.EncodeToString(sum[:]), types.Ptr(int64(len(data)))
	return binary, llm.ReadBytes(data), nil
}
