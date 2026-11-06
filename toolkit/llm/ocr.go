package llm

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ocr is the tool which recognizes the text in an image, or a document,
// which is a resource, by its URI, with a model, and returns it, as Markdown
type ocr struct {
	toolkit.Base
	models ModelsFunc
	model  string
}

// ocrInput is the input of the ocr tool
type ocrInput struct {
	URI   string `json:"uri" help:"URI of the image, or document, such as a PDF, which is resource://<hash>, as an attachment, or a tool's result, says"`
	Model string `json:"model,omitempty" help:"Model which reads the text, by its name, such as mistral:mistral-ocr-latest, or the default, when it's omitted" optional:""`
	Pages string `json:"pages,omitempty" help:"Pages of a document to read, from zero, such as 0,2-4, or all of them, when it's omitted" optional:""`
}

// ocrOutput is the text of the image, or document, as Markdown, with the
// pages it's of
type ocrOutput struct {
	Text  string `json:"text"`
	Pages int    `json:"pages"`
}

var _ gollm.Tool = (*ocr)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*ocr) Name() string {
	return "ocr"
}

func (t *ocr) Description() string {
	return withDefault("Reads the text in an image, or a document, such as a scanned page, or a PDF, which is an attachment, or a tool's result, by its URI, resource://<hash>, and returns it as Markdown. Use it to read a document's text exactly, rather than describing it.", t.model)
}

func (*ocr) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[ocrInput]()
}

// Run reads the resource with the URI, in the session, and returns its
// text, as Markdown, with its pages separated by rules, saying what it's
// doing, and the pages, and tokens, it used
func (t *ocr) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req ocrInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, gollm.ErrBadParameter.With(err.Error())
	}
	models, model, err := modelFor(ctx, t.models, req.Model, t.model)
	if err != nil {
		return nil, err
	}
	resource, err := toolkit.Resource(ctx, strings.TrimSpace(req.URI))
	if err != nil {
		return nil, err
	}
	var opts []gollm.Opt
	if pages := strings.TrimSpace(req.Pages); pages != "" {
		opts = append(opts, gollm.WithOpt(gollm.PagesKey, pages))
	}

	toolkit.Logf(ctx, "Reading %s with %s", nameOf(resource), model.Name())
	document, usage, err := models.Recognize(ctx, model, resource, opts...)
	if err != nil {
		return nil, err
	}
	toolkit.SessionFromContext(ctx).Usage(usage)
	texts := make([]string, 0, len(document.Pages))
	for _, page := range document.Pages {
		texts = append(texts, strings.TrimSpace(page.Text))
	}
	return ocrOutput{Text: strings.Join(texts, "\n\n---\n\n"), Pages: len(document.Pages)}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// modelFor returns the models, and the model with the name which the model
// which calls the tool gives, or otherwise the default, or an error when
// there are no models yet, there's neither, or there's no model with the
// name
func modelFor(ctx context.Context, fn ModelsFunc, given, defaultModel string) (Models, gollm.Model, error) {
	models := fn()
	if models == nil {
		return nil, nil, gollm.ErrServiceUnavailable.With("no models yet")
	}
	name := strings.TrimSpace(given)
	if name == "" {
		name = defaultModel
	}
	if name == "" {
		return nil, nil, gollm.ErrBadParameter.With("missing model, which this tool has no default for, so give one")
	}
	model, err := models.GetModel(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	return models, model, nil
}

// withDefault returns the description, with the model the tool uses, unless
// it's given another, when it has one, or that a model must be given
func withDefault(description, model string) string {
	if model == "" {
		return description + " Give the model to use, by its name, as there's no default."
	}
	return description + " It uses " + model + ", unless another model is given."
}

// nameOf returns the resource's name, or its URI, when it has none
func nameOf(resource gollm.Resource) string {
	if name := resource.Name(); name != "" {
		return name
	}
	return resource.URI()
}
