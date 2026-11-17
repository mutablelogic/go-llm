package httphandler

import (
	_ "embed"
	"net/http"

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
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func workspaceListHandlers(workspaces *workspace.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/workspace", nil, func(path httprequest.PathItem) {
		path.Tag(taggroup.Workspaces)
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), workspaces.ListWorkspaces())
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
			if ws, err := workspaces.GetWorkspace(name); err != nil {
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
