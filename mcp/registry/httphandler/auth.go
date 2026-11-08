package httphandler

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AuthorizeRequest is the request to authorize a connector's server, with
// the URL its authorization server redirects to, once a person has signed
// in, which is the API's callback, of the request's origin, when it's
// omitted
type AuthorizeRequest struct {
	Redirect string `json:"redirect,omitempty" help:"The URL the authorization server redirects to, once a person has signed in, which is the API's callback, of the request's origin, when it's omitted" format:"uri" optional:""`
}

// AuthorizeResponse is the URL a person signs in at, to authorize the
// connector's server
type AuthorizeResponse struct {
	URL string `json:"url" help:"The URL a person signs in at, to authorize the connector's server" format:"uri" readonly:""`
}

// callbackQuery is the query the authorization server redirects to the
// callback with
type callbackQuery struct {
	State            string `json:"state" help:"The state of the authorization, from its URL"`
	Code             string `json:"code,omitempty" help:"The code, which is exchanged for a token" optional:""`
	Error            string `json:"error,omitempty" help:"Why it wasn't authorized, such as access_denied" optional:""`
	ErrorDescription string `json:"error_description,omitempty" help:"Why it wasn't authorized, for a person" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// callbackPage is the page a person is shown, once they're redirected to
// the callback
var callbackPage = template.Must(template.New("callback").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{ .Title }}</title>
<style>body{font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem;line-height:1.5}</style></head>
<body><h1>{{ .Title }}</h1><p>{{ .Message }}</p></body>
</html>
`))

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func authorizeHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return errors.Join(
		router.Register("mcp/connector/{name}/authorize", jsonschema.MustFor[connectorName](), func(path httprequest.PathItem) {
			path.Tag(Tag)

			// POST
			path.Post(func(w http.ResponseWriter, r *http.Request) {
				name := r.PathValue("name")
				var req AuthorizeRequest
				if r.ContentLength != 0 {
					if err := httprequest.Read(r, &req); err != nil {
						httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
						return
					}
				}
				redirect := req.Redirect
				if redirect == "" {
					origin, err := requestOrigin(r)
					if err != nil {
						httpresponse.Error(w, llm.HTTPErr(err), name)
						return
					}
					redirect = origin + strings.TrimSuffix(router.Prefix(), "/") + registry.CallbackPath
				}
				if authorizeURL, err := reg.Authorize(r.Context(), name, redirect); err != nil {
					httpresponse.Error(w, llm.HTTPErr(err), name)
				} else {
					httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), AuthorizeResponse{URL: authorizeURL})
				}
			}, func(op httprequest.PathOperation) {
				op.Summary("Authorize Connector")
				op.Description(documentation.Section(3, "POST /mcp/connector/{name}/authorize").Body)
				op.RequestBody(jsonschema.MustFor[AuthorizeRequest]())
				op.JSONResponse(http.StatusOK, jsonschema.MustFor[AuthorizeResponse](), "The URL a person signs in at")
				op.ErrorResponse(http.StatusBadRequest, "Invalid redirect, or the request has no origin")
				op.ErrorResponse(http.StatusNotFound, "Connector not found")
				op.ErrorResponse(http.StatusNotImplemented, "Its authorization server doesn't register clients, and the connector has none")
			})
		}),
		router.Register("mcp/callback", nil, func(path httprequest.PathItem) {
			path.Tag(Tag)

			// GET
			path.Get(func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				reason := q.Get("error")
				if description := q.Get("error_description"); description != "" {
					reason = description
				}
				connector, err := reg.Callback(r.Context(), q.Get("state"), q.Get("code"), reason)
				if wantsJSON(r) {
					if err != nil {
						httpresponse.Error(w, llm.HTTPErr(err))
					} else {
						httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), connector)
					}
					return
				}
				writeCallbackPage(w, connector, err)
			}, func(op httprequest.PathOperation) {
				op.Summary("Connector Callback")
				op.Description(documentation.Section(3, "GET /mcp/callback").Body)
				op.Query(jsonschema.MustFor[callbackQuery]())
				op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Connector](), "The connector, which is authorized, when JSON is accepted, or else a page for a person")
				op.ErrorResponse(http.StatusBadRequest, "It wasn't authorized, or its code couldn't be exchanged")
				op.ErrorResponse(http.StatusNotFound, "No authorization is waited for, with the state, or the connector was deleted")
				op.ErrorResponse(http.StatusConflict, "The connector's server, or client, was changed as it was signed in to")
				for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusNotFound, http.StatusConflict} {
					op.Response(status, "text/html", "A page for a person, which says whether they're signed in")
				}
			})
		}),
	)
}

// requestOrigin returns the origin of the request, from its Origin header,
// as a browser sends, or its Referer, or ErrBadParameter, when it has
// neither
func requestOrigin(r *http.Request) (string, error) {
	for _, value := range []string{r.Header.Get("Origin"), r.Header.Get("Referer")} {
		if value == "" || value == "null" {
			continue
		}
		if u, err := url.Parse(value); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			return u.Scheme + "://" + u.Host, nil
		}
	}
	return "", llm.ErrBadParameter.With("the request has no origin, so its redirect is required")
}

// wantsJSON returns true when the request accepts JSON, rather than HTML
func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}

// writeCallbackPage writes the page a person is shown, once they're
// redirected to the callback
func writeCallbackPage(w http.ResponseWriter, connector *schema.Connector, err error) {
	data := struct{ Title, Message string }{}
	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError
		var code httpresponse.Err
		if errors.As(llm.HTTPErr(err), &code) {
			status = int(code)
		}
		data.Title, data.Message = "Not signed in", err.Error()
	} else {
		data.Title, data.Message = "Signed in to "+connector.Name, "You can close this window."
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = callbackPage.Execute(w, data)
}
