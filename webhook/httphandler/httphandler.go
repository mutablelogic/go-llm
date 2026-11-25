// Package httphandler has the webhooks' HTTP API, which lists, creates,
// gets, changes and deletes them, and lists their calls, and the URL which
// services call
package httphandler

import (
	_ "embed"
	"errors"
	"net/http"
	"strconv"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/webhook/registry"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// EndpointFunc returns the server's public endpoint, such as
// https://llm.example.com/api, which a webhook's URL is of, or empty, when
// it has none, when its URL isn't returned
type EndpointFunc func() string

// serial is the path parameter of a webhook's, or a call's, serial number
type serial struct {
	ID uint64 `json:"id" help:"Serial number"`
}

// token is the path parameter of a webhook's token, which is a secret, so
// the router leaves it out of the log, and the traces
type token struct {
	Token string `json:"token" format:"password" help:"Token of the webhook, which its URL has"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed README.md
var readme []byte

const (
	// Tag is the tag of the webhooks' API, in the "LLM" group
	Tag = "Webhooks"

	// HookTag is the tag of the URL which services call
	HookTag = "Hooks"

	// HookPath is the path of the URL which services call, of a webhook's
	// token, which is after it
	HookPath = "hook"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// RegisterHandlers registers the webhooks' API with the router, at webhook,
// and the URL which services call, at hook, of the endpoint, which a
// webhook's URL is returned of, when it's created
func RegisterHandlers(r *registry.Registry, router *httprouter.Router, endpoint EndpointFunc) error {
	if r == nil {
		return llm.ErrBadParameter.With("missing webhook registry")
	}
	documentation := openapi.ParseMarkdown(readme)
	router.Spec().AddTag(Tag, documentation.Section(2, "Webhook Operations").Body)
	router.Spec().AddTag(HookTag, documentation.Section(2, "Hook Operations").Body)
	router.Spec().AddTagGroup(llm.TagGroup, Tag, HookTag)
	return errors.Join(
		webhooksHandler(r, router, documentation, endpoint),
		webhookHandler(r, router, documentation),
		callsHandler(r, router, documentation),
		callHandler(r, router, documentation),
		hookHandler(r, router, documentation),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func webhooksHandler(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc, endpoint EndpointFunc) error {
	return router.Register("webhook", nil, func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, req *http.Request) {
			var list schema.WebhookListRequest
			if err := httprequest.Query(req.URL.Query(), &list); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if webhooks, err := r.ListWebhooks(req.Context(), list); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(req), webhooks)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Webhooks")
			op.Description(documentation.Section(3, "GET /webhook").Body)
			op.Query(jsonschema.MustFor[schema.WebhookListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.WebhookList](), "The webhooks, newest first, and how many there are")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a session which isn't a UUID")
		})

		// POST
		path.Post(func(w http.ResponseWriter, req *http.Request) {
			var meta schema.WebhookMeta
			if err := httprequest.Read(req, &meta); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if webhook, err := r.CreateWebhook(req.Context(), meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				webhook.URL = hookURL(endpoint, webhook.Token)
				httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(req), webhook)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Create Webhook")
			op.Description(documentation.Section(3, "POST /webhook").Body)
			op.RequestBody(jsonschema.MustFor[schema.WebhookMeta]())
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.WebhookToken](), "The webhook, with its token, and URL, which aren't returned again")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a webhook without a session, or a type which isn't a protocol")
		})
	})
}

func webhookHandler(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("webhook/{id}", jsonschema.MustFor[serial](), func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, req *http.Request) {
			if id, err := parseSerial(req, "webhook"); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if webhook, err := r.GetWebhook(req.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(req), webhook)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Webhook")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a serial number which isn't one")
			op.Description(documentation.Section(3, "GET /webhook/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Webhook](), "The webhook")
			op.ErrorResponse(http.StatusNotFound, "Webhook not found")
		})

		// PATCH
		path.Patch(func(w http.ResponseWriter, req *http.Request) {
			var change schema.WebhookRequest
			if id, err := parseSerial(req, "webhook"); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if err := httprequest.Read(req, &change); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if webhook, err := r.UpdateWebhook(req.Context(), id, change); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(req), webhook)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Update Webhook")
			op.Description(documentation.Section(3, "PATCH /webhook/{id}").Body)
			op.RequestBody(jsonschema.MustFor[schema.WebhookRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Webhook](), "The webhook, as it's changed")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusNotFound, "Webhook not found")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, req *http.Request) {
			if id, err := parseSerial(req, "webhook"); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if webhook, err := r.DeleteWebhook(req.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(req), webhook)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Webhook")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a serial number which isn't one")
			op.Description(documentation.Section(3, "DELETE /webhook/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Webhook](), "The webhook, as it was")
			op.ErrorResponse(http.StatusNotFound, "Webhook not found")
		})
	})
}

func callsHandler(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("webhook/call", nil, func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, req *http.Request) {
			var list schema.CallListRequest
			if err := httprequest.Query(req.URL.Query(), &list); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if calls, err := r.ListCalls(req.Context(), list); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(req), calls)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Calls")
			op.Description(documentation.Section(3, "GET /webhook/call").Body)
			op.Query(jsonschema.MustFor[schema.CallListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.CallList](), "The calls, oldest first, or newest, and how many there are after the call")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a status which isn't one")
		})

		// POST
		path.Post(func(w http.ResponseWriter, req *http.Request) {
			var test schema.TestRequest
			if err := httprequest.Read(req, &test); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if call, err := r.TestCall(req.Context(), test); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(req), call)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Test Webhook")
			op.Description(documentation.Section(3, "POST /webhook/call").Body)
			op.RequestBody(jsonschema.MustFor[schema.TestRequest]())
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.Call](), "The test call, which was accepted, or refused")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a good signature, of a webhook which has no secret")
			op.ErrorResponse(http.StatusNotFound, "Webhook not found")
		})

		// PATCH
		path.Patch(func(w http.ResponseWriter, req *http.Request) {
			var seen schema.SeenRequest
			if err := httprequest.Read(req, &seen); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if response, err := r.SeeCalls(req.Context(), seen); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(req), response)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("See Calls")
			op.Description(documentation.Section(3, "PATCH /webhook/call").Body)
			op.RequestBody(jsonschema.MustFor[schema.SeenRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.SeenResponse](), "How many calls weren't seen before")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as without calls")
		})
	})
}

func callHandler(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("webhook/call/{id}", jsonschema.MustFor[serial](), func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, req *http.Request) {
			if id, err := parseSerial(req, "call"); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if call, err := r.GetCall(req.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(req), call)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Call")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a serial number which isn't one")
			op.Description(documentation.Section(3, "GET /webhook/call/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Call](), "The call")
			op.ErrorResponse(http.StatusNotFound, "Call not found")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, req *http.Request) {
			if id, err := parseSerial(req, "call"); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if call, err := r.DeleteCall(req.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(req), call)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Call")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a serial number which isn't one")
			op.Description(documentation.Section(3, "DELETE /webhook/call/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Call](), "The call, as it was")
			op.ErrorResponse(http.StatusNotFound, "Call not found")
		})
	})
}

func hookHandler(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register(HookPath+"/{token}", jsonschema.MustFor[token](), func(path httprequest.PathItem) {
		path.Tag(HookTag)
		handler := func(w http.ResponseWriter, req *http.Request) {
			received, err := r.Call(req.Context(), req.PathValue("token"), req)
			switch {
			case err != nil:
				httpresponse.Error(w, llm.HTTPErr(err))
			case received.IsHandshake:
				w.Header().Set(types.ContentTypeHeader, types.ContentTypeTextPlain)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(received.Handshake)
			case received.Duplicate:
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(req), schema.HookResponse{Duplicate: true})
			default:
				httpresponse.JSON(w, http.StatusAccepted, httprequest.Indent(req), schema.HookResponse{Call: received.Call.ID})
			}
		}
		describe := func(method string) func(op httprequest.PathOperation) {
			return func(op httprequest.PathOperation) {
				op.Summary("Call Webhook (" + method + ")")
				op.Description(documentation.Section(3, "POST /hook/{token}").Body)
				op.JSONResponse(http.StatusAccepted, jsonschema.MustFor[schema.HookResponse](), "The call, which was kept")
				op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.HookResponse](), "A delivery which was sent again, or the response to a handshake")
				op.ErrorResponse(http.StatusBadRequest, "Its body is too large, or isn't what its content type says")
				op.ErrorResponse(http.StatusForbidden, "It isn't signed with the webhook's secret")
				op.ErrorResponse(http.StatusNotFound, "No webhook has the token")
			}
		}
		path.Post(handler, describe(http.MethodPost))
		path.Put(handler, describe(http.MethodPut))
		path.Get(handler, describe(http.MethodGet))
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// parseSerial returns the serial number of the path, of what it's of, such
// as a webhook
func parseSerial(req *http.Request, of string) (uint64, error) {
	id, err := strconv.ParseUint(req.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, llm.ErrBadParameter.Withf("invalid %s id %q", of, req.PathValue("id"))
	}
	return id, nil
}

// hookURL returns the URL of the token, of the endpoint, or empty, when
// there's none
func hookURL(endpoint EndpointFunc, token string) string {
	if endpoint == nil {
		return ""
	}
	base := strings.TrimSuffix(strings.TrimSpace(endpoint()), "/")
	if base == "" {
		return ""
	}
	return base + "/" + HookPath + "/" + token
}
