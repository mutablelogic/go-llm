package httphandler

import (
	"context"
	"encoding/json"
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

// transcribeFn transcribes, or translates, the speech in audio, with a model,
// and streams it to the function, when it isn't nil
type transcribeFn func(context.Context, llm.Model, llm.Resource, llm.TranscriberStreamFn, ...llm.Opt) (llm.Transcript, llm.Usage, error)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func transcribeHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return errors.Join(
		transcribeHandler(reg, router, documentation, "transcribe", "Transcribe", reg.Transcribe),
		transcribeHandler(reg, router, documentation, "translate", "Translate", reg.Translate),
	)
}

// transcribeHandler registers the handler of POST /llm/{path}, which asks a
// model to transcribe, or translate, the speech in audio, with the function
func transcribeHandler(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc, path, summary string, fn transcribeFn) error {
	return router.Register("llm/"+path, nil, func(item httprequest.PathItem) {
		item.Tag("Inference")

		// POST, with JSON, or a form, asking a model for the speech in audio,
		// which is streamed, as server-sent events, when they're accepted:
		// a text event for each piece of its text, and a segment event for
		// each segment, when they're asked for
		item.Post(func(w http.ResponseWriter, r *http.Request) {
			// The body is limited, as the audio is read into memory
			if err := limitBody(w, r, schema.TranscribeBodyLimit); err != nil {
				httpresponse.Error(w, err)
			} else if req, err := readTranscribe(r); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				respond(w, r, req.Model, func(write streamWriteFn) (*schema.TranscribeResponse, error) {
					var stream llm.TranscriberStreamFn
					if write != nil {
						stream = func(text string, segment *llm.TranscriptSegment) {
							if segment != nil {
								write(schema.EventSegment, segment)
							} else {
								write(schema.EventText, schema.StreamEvent{Text: text})
							}
						}
					}
					return transcribe(r, reg, *req, fn, stream)
				})
			}
		}, func(op httprequest.PathOperation) {
			op.Summary(summary)
			op.Description(documentation.Section(3, "POST /llm/"+path).Body)
			op.RequestBody(jsonschema.MustFor[schema.TranscribeRequest]())
			op.RequestBody(jsonschema.MustFor[schema.TranscribeForm](), types.ContentTypeFormData)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.TranscribeResponse](), "The transcript, its language, when it's known, and its segments, when they're asked for, the model which "+path+"d it, and the seconds and tokens used, or a stream of server-sent events when text/event-stream is accepted")
			op.TextStreamResponse(http.StatusOK)
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body or form, no model, type, data or file, more than one file, invalid options, or audio the provider can't "+path+", or segments or speakers for a model which can't return them")
			op.ErrorResponse(http.StatusNotFound, "Model not found")
			op.ErrorResponse(http.StatusNotAcceptable, "The response can't be any of the accepted types")
			op.ErrorResponse(http.StatusConflict, "More than one provider has a model with the name")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The request body is larger than 48 MiB")
			op.ErrorResponse(http.StatusNotImplemented, "The model's provider can't "+path+" speech, or the model is known not to")
			op.ErrorResponse(http.StatusServiceUnavailable, "The model couldn't "+path+" the speech, or a provider which may have it can't be reached")
		})
	})
}

// transcribe returns the speech in the audio of the request, from its model,
// which is found by its name, with the function, which streams it, when the
// stream function isn't nil, with the model's canonical name, and the
// seconds and tokens used
func transcribe(r *http.Request, reg *registry.Registry, req schema.TranscribeRequest, fn transcribeFn, stream llm.TranscriberStreamFn) (*schema.TranscribeResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	}
	audio, err := req.Resource()
	if err != nil {
		return nil, err
	}

	// Transcribe the speech, with the model
	model, err := reg.GetModel(r.Context(), strings.TrimSpace(req.Model))
	if err != nil {
		return nil, err
	}
	transcript, usage, err := fn(r.Context(), model, audio, stream, req.Opts()...)
	if err != nil {
		return nil, err
	}
	return &schema.TranscribeResponse{
		Model:      reg.Name(model),
		Transcript: transcript,
		Usage:      usage,
	}, nil
}

// readTranscribe reads the request to transcribe speech, which is JSON, or a
// form, with the audio uploaded as one file, named "file", as with
// readRecognize
func readTranscribe(r *http.Request) (*schema.TranscribeRequest, error) {
	if contentType, _ := types.ParseContentType(r.Header.Get(types.ContentTypeHeader)); contentType != types.ContentTypeFormData {
		var req schema.TranscribeRequest
		if err := httprequest.Read(r, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("%v", err)
		}
		return types.Ptr(req), nil
	}

	// Read the form, with the file
	var form schema.TranscribeForm
	file, err := readForm(r, &form)
	if err != nil {
		return nil, err
	} else if file == nil {
		return nil, llm.ErrBadParameter.With("no file to transcribe")
	}

	// The options are a JSON object
	req := schema.TranscribeRequest{Model: form.Model, Type: file.Type, Data: file.Data}
	if opt := strings.TrimSpace(form.Opt); opt != "" {
		if err := json.Unmarshal([]byte(opt), &req.Opt); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid options: %v", err)
		}
	}
	return types.Ptr(req), nil
}
