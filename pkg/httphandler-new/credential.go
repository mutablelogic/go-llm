package httphandler

import (
	"context"
	"net/http"

	// Packages
	llmmanager "github.com/mutablelogic/go-llm/pkg/llmmanager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func CredentialHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "credential", nil, httprequest.NewPathItem(
		"Credential operations",
		"Create operations on credentials",
		"Connector",
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			_ = createCredential(r.Context(), manager, w, r)
		},
		"Create credential",
		opts.WithJSONRequest(jsonschema.MustFor[schema.CredentialInsert]()),
		opts.WithJSONResponse(201, jsonschema.MustFor[schema.Credential]()),
		opts.WithErrorResponse(400, "Invalid request body or credential creation failure."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func createCredential(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.CredentialInsert
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	credential, err := manager.CreateCredential(ctx, req)
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), credential)
}
