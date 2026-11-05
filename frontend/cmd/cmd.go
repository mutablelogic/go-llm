//go:build !client

// Package cmd serves the web UI, and its configuration, alongside the API,
// when the server is built with the "frontend" tag, which embeds the UI,
// once it's built, with npm, into build/frontend, which make does. Without
// it, such as with go install, the server serves the API, without the UI.
package cmd

import (
	"context"
	"log/slog"
	"net/http"
	"path"

	// Packages
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	version "github.com/mutablelogic/go-server/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the web UI's flags
type Flags struct {
	UI bool `name:"ui" help:"Serve the web UI from this server, when it's built in." default:"true" negatable:""`
}

// Config is the configuration of the web UI, which it reads from
// /config.json
type Config struct {
	Prefix  string `json:"prefix" help:"Path prefix of the API, such as /api"`
	Version string `json:"version,omitempty" help:"Version of the server"`
	OpenAPI string `json:"openapi,omitempty" help:"Path of the API's documentation, when it's served"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// RegisterUI registers the web UI, when it's built in, and its
// configuration, unless the UI isn't to be served. When openapi is set, the
// server serves the API's documentation, at {prefix}/openapi.html, which the
// UI links to.
func (f *Flags) RegisterUI(ctx context.Context, log *slog.Logger, router *httprouter.Router, openapi bool) error {
	if !f.UI {
		return nil
	} else if !builtIn {
		log.DebugContext(ctx, "the web UI isn't built in, so isn't served, which make, with npm, builds it into")
		return nil
	}
	log.DebugContext(ctx, "registering web UI handlers")
	if err := registerConfig(router, openapi); err != nil {
		return err
	}
	return registerFS(router)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// registerConfig registers /config.json, which is the configuration of the
// web UI, so it knows the prefix of the API, which can be changed with
// --http.prefix, and where the API's documentation is, when it's served
func registerConfig(router *httprouter.Router, openapi bool) error {
	config := Config{Prefix: router.Prefix(), Version: version.Version()}
	if openapi {
		config.OpenAPI = path.Join(router.Prefix(), "openapi.html")
	}
	return router.Register("/config.json", nil, func(path httprequest.PathItem) {
		path.Tag("UI")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), config)
		}, func(op httprequest.PathOperation) {
			op.Summary("Get UI Configuration")
			op.Description("The configuration of the web UI, which is the path prefix of the API, the version of the server, and the path of the API's documentation, when it's served.")
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[Config](), "Configuration")
		})
	})
}
