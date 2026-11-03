package httphandler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
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

func embedHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	router.Spec().AddTag("Inference", documentation.Section(2, "Inference Operations").Body)
	return router.Register("llm/embed", nil, func(path httprequest.PathItem) {
		path.Tag("Inference")

		// POST, with JSON, asking a model for a vector for each of the
		// messages, or a form, for one message
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			// The body is limited, as messages' images and audio are read
			// into memory
			if err := limitBody(w, r, schema.EmbedBodyLimit); err != nil {
				httpresponse.Error(w, err)
			} else if req, err := readEmbed(r); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else if response, err := embed(r, reg, *req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.Model)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), response)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Embed")
			op.Description(documentation.Section(3, "POST /llm/embed").Body)
			op.RequestBody(jsonschema.MustFor[schema.EmbedRequest]())
			op.RequestBody(jsonschema.MustFor[schema.EmbedForm](), types.ContentTypeFormData)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.EmbedResponse](), "A vector for each of the messages, the model which embedded them, and the tokens used")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body or form, no messages or model, no text, no file, more than one file, invalid options, or an option or message the provider can't embed")
			op.ErrorResponse(http.StatusNotFound, "Model not found")
			op.ErrorResponse(http.StatusConflict, "More than one provider has a model with the name")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The request body is larger than 32 MiB")
			op.ErrorResponse(http.StatusNotImplemented, "The model's provider can't embed messages, or the model is known not to, or its server has no embeddings, such as DeepSeek")
			op.ErrorResponse(http.StatusServiceUnavailable, "The model couldn't embed the messages, or a provider which may have it can't be reached")
		})
	})
}

// embed returns a vector for each of the messages in the request, from its
// model, which is found by its name, with the model's canonical name, and
// the tokens used
func embed(r *http.Request, reg *registry.Registry, req schema.EmbedRequest) (*schema.EmbedResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	} else if len(req.Messages) == 0 {
		return nil, llm.ErrBadParameter.With("missing messages")
	}

	// The messages
	messages := make([]llm.Message, 0, len(req.Messages))
	for i, message := range req.Messages {
		m, err := message.Message()
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("message %d: %v", i, err)
		}
		messages = append(messages, m)
	}

	// Embed them, with the model
	model, err := reg.GetModel(r.Context(), strings.TrimSpace(req.Model))
	if err != nil {
		return nil, err
	}
	vectors, usage, err := reg.Embed(r.Context(), model, messages, req.Opts()...)
	if err != nil {
		return nil, err
	}
	return &schema.EmbedResponse{
		Model:   reg.Name(model),
		Vectors: vectors,
		Usage:   usage,
	}, nil
}

// readEmbed reads the request to embed messages, which is JSON, or a form
// to embed one message, which is text, an image or audio uploaded as one
// file, named "file", as with readRecognize, which is read as a request
// with the message. A file whose type is text, such as text/plain, is the
// message's text.
func readEmbed(r *http.Request) (*schema.EmbedRequest, error) {
	if contentType, _ := types.ParseContentType(r.Header.Get(types.ContentTypeHeader)); contentType != types.ContentTypeFormData {
		var req schema.EmbedRequest
		if err := httprequest.Read(r, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("%v", err)
		}
		return types.Ptr(req), nil
	}

	// Read the form, with the file
	var form schema.EmbedForm
	file, err := readForm(r, &form)
	if err != nil {
		return nil, err
	} else if file == nil {
		return nil, llm.ErrBadParameter.With("no file to embed")
	}

	// The message is the file's text, when it's text, or otherwise the file
	// as an attachment
	var message schema.EmbedMessage
	if mediaType, _, _ := mime.ParseMediaType(file.Type); strings.HasPrefix(mediaType, "text/") {
		message.Content = append(message.Content, schema.EmbedContent{Text: types.Ptr(string(file.Data))})
	} else {
		message.Content = append(message.Content, schema.EmbedContent{Attachment: &schema.EmbedAttachment{Type: file.Type, Data: file.Data}})
	}

	// The options are a JSON object
	req := schema.EmbedRequest{Model: form.Model, Messages: []schema.EmbedMessage{message}}
	if opt := strings.TrimSpace(form.Opt); opt != "" {
		if err := json.Unmarshal([]byte(opt), &req.Opt); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid options: %v", err)
		}
	}
	return types.Ptr(req), nil
}

// formFile is the file uploaded with a form, by its type and data
type formFile struct {
	Type string
	Data []byte
}

// readForm reads the form into the value, with at most one file, named
// "file", which is streamed from the request, and returned, or nil when
// there's no file. A file without a name is a value in the form. The file's
// type is used, unless it's the generic application/octet-stream, when the
// type is detected from the data. Returns ErrBadParameter when the form
// isn't valid, or the file is empty.
func readForm(r *http.Request, v any) (*formFile, error) {
	var file *types.File
	var data []byte
	values, err := httprequest.Binary(r, func(field string, _ url.Values, f types.File) (err error) {
		if field != "file" {
			return llm.ErrBadParameter.Withf("unexpected file in field %q", field)
		} else if file != nil {
			return llm.ErrBadParameter.With("more than one file")
		}
		file = types.Ptr(f)
		if data, err = io.ReadAll(f.Body); err != nil {
			return llm.ErrBadParameter.Withf("cannot read file %q: %v", f.Path, err)
		}
		return nil
	})
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("%v", err)
	}
	if value, exists := values["file"]; exists {
		if file != nil || len(value) > 1 {
			return nil, llm.ErrBadParameter.With("more than one file")
		}
		file, data = &types.File{}, []byte(value[0])
		delete(values, "file")
	}
	if err := httprequest.Query(values, v); err != nil {
		return nil, llm.ErrBadParameter.Withf("%v", err)
	}

	// The file, by its type, and data
	if file == nil {
		return nil, nil
	} else if len(data) == 0 {
		return nil, llm.ErrBadParameter.With("file is empty")
	}
	contentType := file.ContentType
	if providerschema.UnknownType(contentType) {
		contentType = providerschema.ResourceType(data, file.Path)
	}
	return &formFile{Type: contentType, Data: data}, nil
}

// limitBody reads the request body, which must be no larger than the limit,
// so it can be decoded. Returns an error with the status 413 if it's larger.
func limitBody(w http.ResponseWriter, r *http.Request, limit int64) error {
	var tooLarge *http.MaxBytesError
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if errors.As(err, &tooLarge) {
		return httpresponse.Err(http.StatusRequestEntityTooLarge).Withf("request body is larger than %d bytes", tooLarge.Limit)
	} else if err != nil {
		return httpresponse.ErrBadRequest.Withf("%v", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return nil
}
