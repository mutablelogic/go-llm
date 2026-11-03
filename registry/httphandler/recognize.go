package httphandler

import (
	"encoding/json"
	"net/http"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
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
// PRIVATE METHODS

func recognizeHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/recognize", nil, func(path httprequest.PathItem) {
		path.Tag("Inference")

		// POST, with JSON, or a form, asking a model for the text in an
		// image or document
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			// The body is limited, as the image or document is read into
			// memory
			if err := limitBody(w, r, schema.RecognizeBodyLimit); err != nil {
				httpresponse.Error(w, err)
			} else if req, err := readRecognize(r); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if response, err := recognize(r, reg, *req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.Model)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), response)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Recognize")
			op.Description(documentation.Section(3, "POST /llm/recognize").Body)
			op.RequestBody(jsonschema.MustFor[schema.RecognizeRequest]())
			op.RequestBody(jsonschema.MustFor[schema.RecognizeForm](), types.ContentTypeFormData)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.RecognizeResponse](), "The text, with a text block for each page, the model which recognized it, and the pages and tokens used")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body or form, no model, type, data or file, more than one file, invalid options, or an image or document the provider can't recognize, such as a PDF for Ollama")
			op.ErrorResponse(http.StatusNotFound, "Model not found")
			op.ErrorResponse(http.StatusConflict, "More than one provider has a model with the name")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The request body is larger than 64 MiB")
			op.ErrorResponse(http.StatusNotImplemented, "The model's provider can't recognize text, or the model is known not to")
			op.ErrorResponse(http.StatusServiceUnavailable, "The model couldn't recognize the text, or a provider which may have it can't be reached")
		})
	})
}

// recognize returns the text in the image or document of the request, from
// its model, which is found by its name, with the model's canonical name,
// and the pages and tokens used
func recognize(r *http.Request, reg *registry.Registry, req schema.RecognizeRequest) (*schema.RecognizeResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	}
	resource, err := req.Resource()
	if err != nil {
		return nil, err
	}

	// Recognize the text, with the model
	model, err := reg.GetModel(r.Context(), strings.TrimSpace(req.Model))
	if err != nil {
		return nil, err
	}
	message, usage, err := reg.Recognize(r.Context(), model, resource, req.Opts()...)
	if err != nil {
		return nil, err
	}
	return &schema.RecognizeResponse{
		Model:   reg.Name(model),
		Message: *providerschema.MessageFrom(message),
		Usage:   usage,
	}, nil
}

// readRecognize reads the request to recognize text, which is JSON, or a
// form, with the image or document uploaded as one file, named "file"
func readRecognize(r *http.Request) (*schema.RecognizeRequest, error) {
	if contentType, _ := types.ParseContentType(r.Header.Get(types.ContentTypeHeader)); contentType != types.ContentTypeFormData {
		var req schema.RecognizeRequest
		if err := httprequest.Read(r, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("%v", err)
		}
		return types.Ptr(req), nil
	}

	// Read the form, with the file
	var form schema.RecognizeForm
	file, err := readForm(r, &form)
	if err != nil {
		return nil, err
	} else if file == nil {
		return nil, llm.ErrBadParameter.With("no file to recognize")
	}

	// The options are a JSON object
	req := schema.RecognizeRequest{Model: form.Model, Type: file.Type, Data: file.Data}
	if opt := strings.TrimSpace(form.Opt); opt != "" {
		if err := json.Unmarshal([]byte(opt), &req.Opt); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid options: %v", err)
		}
	}
	return types.Ptr(req), nil
}
