package httphandler

import (
	"io"
	"mime"
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

func speakHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/speak", nil, func(path httprequest.PathItem) {
		path.Tag("Inference")

		// POST, asking a model to speak text, whose audio is the response
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var req schema.SpeakRequest
			if err := limitBody(w, r, schema.SpeakBodyLimit); err != nil {
				httpresponse.Error(w, err)
			} else if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if err := speak(w, r, reg, req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.Model)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Speak")
			op.Description(documentation.Section(3, "POST /llm/speak").Body)
			op.RequestBody(jsonschema.MustFor[schema.SpeakRequest]())
			op.Response(http.StatusOK, "audio/*", "The spoken text, as audio, with the model and usage as JSON in its X-Meta header")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body, no model or text, or an option the model doesn't support, such as a voice it hasn't")
			op.ErrorResponse(http.StatusNotFound, "Model not found")
			op.ErrorResponse(http.StatusConflict, "More than one provider has a model with the name")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The request body is larger than 1 MiB")
			op.ErrorResponse(http.StatusNotImplemented, "The model's provider can't speak text, or the model is known not to")
			op.ErrorResponse(http.StatusServiceUnavailable, "The model couldn't speak the text, or a provider which may have it can't be reached")
		})
	})
}

// speak writes the text of the request, spoken by its model, which is found
// by its name, as audio, with the model's canonical name, and the usage, in
// its X-Meta header, and its file's name, which is speech, with its
// format's extension
func speak(w http.ResponseWriter, r *http.Request, reg *registry.Registry, req schema.SpeakRequest) error {
	if strings.TrimSpace(req.Model) == "" {
		return llm.ErrBadParameter.With("missing model")
	} else if strings.TrimSpace(req.Text) == "" {
		return llm.ErrBadParameter.With("missing text")
	}
	model, err := reg.GetModel(r.Context(), strings.TrimSpace(req.Model))
	if err != nil {
		return err
	}
	audio, usage, err := reg.Speak(r.Context(), model, req.Text, req.Opts()...)
	if err != nil {
		return err
	}
	read, err := audio.Read(r.Context())
	if err != nil {
		return err
	}

	// The audio is written as it's read, rather than copied, with its size,
	// when it's known, as it is when its data is in memory
	name := "speech"
	if exts, _ := mime.ExtensionsByType(audio.Type()); len(exts) > 0 {
		name += exts[0]
	}
	binary := httpresponse.Binary{
		ContentType: audio.Type(),
		Path:        name,
		Disposition: httpresponse.DispositionInline,
		Meta:        schema.SpeakMeta{Model: reg.Name(model), Usage: usage},
	}
	if inline, ok := audio.(*providerschema.Resource); ok && len(inline.Data) > 0 {
		binary.Size = types.Ptr(int64(len(inline.Data)))
	}
	return read(r.Context(), func(content io.Reader) error {
		return binary.Write(w, r, content)
	})
}
