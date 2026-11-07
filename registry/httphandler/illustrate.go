package httphandler

import (
	"net/http"
	"strings"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func illustrateHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/illustrate", nil, func(path httprequest.PathItem) {
		path.Tag("Inference")

		// POST, asking a model to make images
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var req schema.IllustrateRequest
			if err := limitBody(w, r, schema.IllustrateBodyLimit); err != nil {
				httpresponse.Error(w, err)
			} else if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if response, err := illustrate(r, reg, req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.Model)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), response)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Illustrate")
			op.Description(documentation.Section(3, "POST /llm/illustrate").Body)
			op.RequestBody(jsonschema.MustFor[schema.IllustrateRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.IllustrateResponse](), "The images the model made, as resources, with the model and usage")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body, no model or prompt, an image which isn't one, an option the model doesn't support, or the model made no image")
			op.ErrorResponse(http.StatusNotFound, "Model, an image's resource, or the session, not found")
			op.ErrorResponse(http.StatusConflict, "More than one provider has a model with the name")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The request body is larger than 64 MiB")
			op.ErrorResponse(http.StatusNotImplemented, "The model's provider can't make images, or the model is known not to")
			op.ErrorResponse(http.StatusServiceUnavailable, "The model couldn't make images, or a provider which may have it can't be reached")
		})
	})
}

// illustrate returns the images the request's model, which is found by its
// name, makes from its prompt, and its images, which are kept as resources
// of its session, or on their own, or, when the registry doesn't keep
// resources, are their data, with the model's canonical name, and the usage
func illustrate(r *http.Request, reg *registry.Registry, req schema.IllustrateRequest) (*schema.IllustrateResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	} else if strings.TrimSpace(req.Prompt) == "" {
		return nil, llm.ErrBadParameter.With("missing prompt")
	}
	session := uuid.Nil()
	if id := strings.TrimSpace(req.Session); id != "" {
		var err error
		if session, err = uuid.Parse(id); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid session %q", id)
		}
	}
	images, err := req.Resources()
	if err != nil {
		return nil, err
	}
	model, err := reg.GetModel(r.Context(), strings.TrimSpace(req.Model))
	if err != nil {
		return nil, err
	}
	made, usage, err := reg.Illustrate(r.Context(), model, req.Prompt, images, req.Opts()...)
	if err != nil {
		return nil, err
	}

	// The images, kept as resources, or their data
	response := &schema.IllustrateResponse{Model: reg.Name(model), Usage: usage, Images: make([]schema.Illustration, 0, len(made))}
	for _, image := range made {
		link, kept, err := reg.Keep(r.Context(), session, image, "")
		if err != nil {
			return nil, err
		}
		if kept {
			response.Images = append(response.Images, schema.Illustration{URI: link.URI, Name: link.Name, Type: link.Type, Size: link.Size})
			continue
		}
		data, err := llm.ReadAll(r.Context(), image)
		if err != nil {
			return nil, err
		}
		response.Images = append(response.Images, schema.Illustration{Type: image.Type(), Size: int64(len(data)), Data: data})
	}
	return response, nil
}
