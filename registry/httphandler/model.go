package httphandler

import (
	"errors"
	"net/http"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// modelName is the path parameter naming a model, which is one segment of
// the path, so a "/" in it is escaped, as %2F
type modelName struct {
	Name string `json:"name" help:"Model name, or alias, or provider and name, separated by a colon, with a / escaped as %2F"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func modelHandlers(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	router.Spec().AddTag("Models", documentation.Section(2, "Model Operations").Body)
	return errors.Join(
		modelListHandlers(r, router, documentation),
		modelGetHandlers(r, router, documentation),
		modelUnloadHandlers(r, router, documentation),
		modelVoiceHandlers(r, router, documentation),
	)
}

func modelListHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/model", nil, func(path httprequest.PathItem) {
		path.Tag("Models")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.ModelListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := listModels(r, reg, req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Models")
			op.Description(documentation.Section(3, "GET /llm/model").Body)
			op.Query(jsonschema.MustFor[schema.ModelListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ModelList](), "List of models")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusNotFound, "Provider not found")
			op.ErrorResponse(http.StatusServiceUnavailable, "No provider's models can be listed")
		})

		// POST, loading a model. The download's progress is streamed as
		// server-sent events, when they're accepted.
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var req schema.LoadModelRequest
			if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
				return
			}
			accept, err := types.AcceptContentType(r)
			if err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
				return
			}
			switch accept {
			case types.ContentTypeTextStream:
				loadModelStream(reg, w, r, req)
			case "", types.ContentTypeJSON, types.ContentTypeAny:
				if model, err := loadModel(r, reg, req, nil); err != nil {
					httpresponse.Error(w, llm.HTTPErr(err), req.String())
				} else {
					httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), model)
				}
			default:
				httpresponse.Error(w, httpresponse.Err(http.StatusNotAcceptable).Withf("cannot respond with %q", accept))
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Load Model")
			op.Description(documentation.Section(3, "POST /llm/model").Body)
			op.RequestBody(jsonschema.MustFor[schema.LoadModelRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Model](), "Loaded model, or a stream of server-sent events when text/event-stream is accepted")
			op.TextStreamResponse(http.StatusOK)
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body, missing model name, or more than one provider loads models, and the request has no provider")
			op.ErrorResponse(http.StatusNotFound, "Provider not found")
			op.ErrorResponse(http.StatusNotAcceptable, "The response can't be any of the accepted types")
			op.ErrorResponse(http.StatusNotImplemented, "No provider loads models, or not the provider in the request")
			op.ErrorResponse(http.StatusServiceUnavailable, "The model couldn't be loaded")
		})
	})
}

func modelGetHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/model/{name}", jsonschema.MustFor[modelName](), func(path httprequest.PathItem) {
		path.Tag("Models")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if model, err := reg.GetModel(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), reg.Model(model))
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Model")
			op.Description(documentation.Section(3, "GET /llm/model/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Model](), "Model")
			op.ErrorResponse(http.StatusNotFound, "Model not found")
			op.ErrorResponse(http.StatusConflict, "More than one provider has the model, so its provider is needed")
			op.ErrorResponse(http.StatusServiceUnavailable, "No provider has the model, and a provider which could can't be reached")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if model, err := deleteModel(r, reg, name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), model)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Model")
			op.Description(documentation.Section(3, "DELETE /llm/model/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Model](), "Deleted model")
			op.ErrorResponse(http.StatusNotFound, "Model not found")
			op.ErrorResponse(http.StatusConflict, "More than one provider has the model, so its provider is needed")
			op.ErrorResponse(http.StatusNotImplemented, "The model's provider can't delete models")
			op.ErrorResponse(http.StatusServiceUnavailable, "The model couldn't be deleted, or a provider which could have it can't be reached")
		})
	})
}

// listModels returns the models of the providers, or of the provider which
// the request selects, which must exist. The models of a provider which
// can't be listed are left out, unless no models can be listed, when it's
// the error.
func listModels(r *http.Request, reg *registry.Registry, req schema.ModelListRequest) (*schema.ModelList, error) {
	// The capabilities are checked first, so a request for one which isn't
	// is refused, rather than the providers being asked
	if err := llm.ValidCapabilities(req.Capability...); err != nil {
		return nil, err
	}
	var providers []string
	if req.Provider != "" {
		providers = append(providers, req.Provider)
	}
	models, err := reg.ListModels(r.Context(), providers...)
	if err != nil && len(models) == 0 {
		return nil, err
	}
	if models, err = llm.WithCapabilities(models, req.Capability...); err != nil {
		return nil, err
	}
	list := &schema.ModelList{ModelListRequest: req, Body: make([]*schema.Model, 0, len(models))}
	for _, model := range models {
		list.Body = append(list.Body, reg.Model(model))
	}
	list.Count = uint(len(list.Body))
	return list, nil
}

// modelUnloadHandlers registers the path which unloads a model
func modelUnloadHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/model/{name}/unload", jsonschema.MustFor[modelName](), func(path httprequest.PathItem) {
		path.Tag("Models")

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if model, err := unloadModel(r, reg, name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), model)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Unload Model")
			op.Description(documentation.Section(3, "POST /llm/model/{name}/unload").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Model](), "Unloaded model")
			op.ErrorResponse(http.StatusNotFound, "Model not found")
			op.ErrorResponse(http.StatusConflict, "More than one provider has the model, so its provider is needed")
			op.ErrorResponse(http.StatusNotImplemented, "The model's provider doesn't load models, or can't unload them")
			op.ErrorResponse(http.StatusServiceUnavailable, "The model couldn't be unloaded, or a provider which could have it can't be reached")
		})
	})
}

// modelVoiceHandlers registers the path which lists a model's voices
func modelVoiceHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/model/{name}/voice", jsonschema.MustFor[modelName](), func(path httprequest.PathItem) {
		path.Tag("Models")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			var req schema.VoiceListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := listVoices(r, reg, name, req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Voices")
			op.Description(documentation.Section(3, "GET /llm/model/{name}/voice").Body)
			op.Query(jsonschema.MustFor[schema.VoiceListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.VoiceList](), "The model's voices, and the token of the next page")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a limit which isn't a number, or a page token which isn't one")
			op.ErrorResponse(http.StatusNotFound, "Model not found")
			op.ErrorResponse(http.StatusConflict, "More than one provider has the model, so its provider is needed")
			op.ErrorResponse(http.StatusNotImplemented, "The model doesn't speak, or its provider can't speak text")
			op.ErrorResponse(http.StatusServiceUnavailable, "The voices couldn't be listed, or a provider which could have the model can't be reached")
		})
	})
}

// listVoices returns the voices of the model with the name, or alias, or
// provider and name, which the request selects: those whose title, or ID,
// has the voice, at most the limit of them, from the page the token says,
// with the token of the next page
func listVoices(r *http.Request, reg *registry.Registry, name string, req schema.VoiceListRequest) (*schema.VoiceList, error) {
	model, err := reg.GetModel(r.Context(), name)
	if err != nil {
		return nil, err
	}
	var opts []llm.Opt
	if req.Limit != nil {
		opts = append(opts, llm.WithLimit(*req.Limit))
	}
	if req.Next != "" {
		opts = append(opts, llm.WithPageToken(req.Next))
	}

	// A search is of all the voices, which are then paged, as a provider
	// pages its voices, rather than those it finds
	var voices *llm.VoiceList
	if search := strings.TrimSpace(req.Voice); search != "" {
		all, err := reg.ListVoices(r.Context(), model)
		if err != nil {
			return nil, err
		}
		if voices, err = llm.PagedVoices(llm.FindVoices(all.Body, search), opts...); err != nil {
			return nil, err
		}
	} else if voices, err = reg.ListVoices(r.Context(), model, opts...); err != nil {
		return nil, err
	}
	return &schema.VoiceList{Model: reg.Model(model).Canonical, Voice: strings.TrimSpace(req.Voice), Body: voices.Body, Count: voices.Count, Next: voices.Next}, nil
}

// unloadModel unloads the model with the name, or alias, or provider and
// name, from memory, with its provider, and returns it
func unloadModel(r *http.Request, reg *registry.Registry, name string) (*schema.Model, error) {
	model, err := reg.GetModel(r.Context(), name)
	if err != nil {
		return nil, err
	}
	if err := reg.UnloadModel(r.Context(), model); err != nil {
		return nil, err
	}
	return reg.Model(model), nil
}

// loadModel loads the model in the request, with its provider, which must
// exist, or the one provider which loads models, calling the progress
// function, when it isn't nil, as it's downloaded
func loadModel(r *http.Request, reg *registry.Registry, req schema.LoadModelRequest, progress llm.DownloadProgressFn) (*schema.Model, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}
	if provider := strings.TrimSpace(req.Provider); provider != "" {
		if reg.Provider(provider) == nil {
			return nil, llm.ErrNotFound.Withf("provider %q", provider)
		}
		name = provider + ":" + name
	}
	model, err := reg.LoadModel(r.Context(), name, progress)
	if err != nil {
		return nil, err
	}
	return reg.Model(model), nil
}

// loadModelStream loads the model, streaming progress events as it's
// downloaded, and then a result event with the model, or an error event
func loadModelStream(reg *registry.Registry, w http.ResponseWriter, r *http.Request, req schema.LoadModelRequest) {
	stream := httpresponse.NewTextStream(w)
	if stream == nil {
		httpresponse.Error(w, httpresponse.ErrInternalError.With("streaming isn't supported"))
		return
	}
	defer stream.Close()

	model, err := loadModel(r, reg, req, func(status string, percent float64) {
		stream.Write(schema.EventProgress, schema.ProgressEvent{Status: status, Percent: percent})
	})
	if err != nil {
		stream.Write(schema.EventError, schema.ErrorEvent{Error: llm.HTTPErr(err).Error()})
	} else {
		stream.Write(schema.EventResult, model)
	}
}

// deleteModel deletes the model with the name, or alias, or provider and
// name, with its provider, and returns it
func deleteModel(r *http.Request, reg *registry.Registry, name string) (*schema.Model, error) {
	model, err := reg.GetModel(r.Context(), name)
	if err != nil {
		return nil, err
	}
	result := reg.Model(model)
	if err := reg.DeleteModel(r.Context(), model); err != nil {
		return nil, err
	}
	return result, nil
}
