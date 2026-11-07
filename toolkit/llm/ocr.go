package llm

import (
	"context"
	"encoding/json"
	"mime"
	"strings"
	"time"

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
	Name  string `json:"name,omitempty" help:"Name of a Markdown file the text is also kept in, such as report, which .md is added to, or the document's name, when only a title is given" optional:""`
	Title string `json:"title,omitempty" help:"Title of a Markdown file the text is also kept in, for people, such as The quarterly report. With a name, or a title, the text is kept as a file of the chat, as well as returned" optional:""`
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
	return withDefault("Reads the text in an image, or a document, such as a scanned page, or a PDF, which is an attachment, or a tool's result, by its URI, resource://<hash>, and returns it as Markdown. Use it to read a document's text exactly, rather than describing it.", t.model, "ocr")
}

func (*ocr) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[ocrInput]()
}

// Timeout returns the most time the tool has, which is longer than a tool
// has by default, for reading a long document, page by page
func (*ocr) Timeout() time.Duration {
	return ocrTimeout
}

// Run reads the resource with the URI, in the session, and returns its
// text, as Markdown, with its pages separated by rules, saying what it's
// doing, and the pages, and tokens, it used
func (t *ocr) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req ocrInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, gollm.ErrBadParameter.With(err.Error())
	}
	models, model, err := modelFor(ctx, t.models, req.Model, t.model, "ocr")
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

	if pages := strings.TrimSpace(req.Pages); pages != "" {
		toolkit.Logf(ctx, "Reading the text on pages %s of %s", pages, describe(resource))
	} else {
		toolkit.Logf(ctx, "Reading the text in %s", describe(resource))
	}
	document, usage, err := models.Recognize(ctx, model, resource, opts...)
	if err != nil {
		return nil, err
	}
	toolkit.SessionFromContext(ctx).Usage(usage)
	texts := make([]string, 0, len(document.Pages))
	for _, page := range document.Pages {
		texts = append(texts, strings.TrimSpace(page.Text))
	}
	output := ocrOutput{Text: strings.Join(texts, "\n\n---\n\n"), Pages: len(document.Pages)}
	if strings.TrimSpace(req.Name) == "" && strings.TrimSpace(req.Title) == "" {
		return output, nil
	}
	file := textFile(output.Text, "text/markdown; charset=utf-8", ".md", req.Name, req.Title, resource, "text")
	return toolkit.Result{Value: output, Resources: []gollm.Resource{file}}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// modelFor returns the models, and the model with the name which the model
// which calls the tool gives, or otherwise the default, or an error when
// there are no models yet, there's neither, or there's no model with the
// name
func modelFor(ctx context.Context, fn ModelsFunc, given, defaultModel, capability string) (Models, gollm.Model, error) {
	models := fn()
	if models == nil {
		return nil, nil, gollm.ErrServiceUnavailable.With("no models yet")
	}
	name := strings.TrimSpace(given)
	if name == "" {
		name = defaultModel
	}
	if name == "" {
		return nil, nil, gollm.ErrBadParameter.Withf("missing model: give the one the person chose for %s, which is in the memory %s, or list the models with the %s capability with the models tool, and ask them which to use, unless only one has it, and remember it there", capability, PreferredModels, capability)
	}
	model, err := models.GetModel(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	return models, model, nil
}

// PreferredModels is the key of the memory which has the models the person
// chose for each capability, such as ocr, which the tools' descriptions tell
// the model which calls them to recall, and remember
const PreferredModels = "preferred_models"

// withDefault returns the description, with the model the tool uses, unless
// it's given another, when it has one, or how the model to give is chosen:
// the person's choice, which is remembered, or else one with the capability
// the tool needs, which the models tool lists, which the person is asked to
// choose from
func withDefault(description, model, capability string) string {
	if model != "" {
		return description + " It uses " + model + ", unless another model is given, which the models tool lists, by the " + capability + " capability."
	}
	return description + " Give the model to use, by its name. Use the one the person chose for " + capability + ": when you can remember things, recall the memory " + PreferredModels + ", which has the models they chose. When they haven't chosen one, list the models with the " + capability + " capability with the models tool, and ask them which to use, unless only one has it, and remember their choice, or that one, in " + PreferredModels + ", as a line such as \"- " + capability + ": <model>\", keeping the other lines."
}

// describe returns what the resource is, for what a tool says it's doing:
// its name, such as scan.pdf, or, without one, what it is, by its type, such
// as the image, or the recording, rather than its URI
func describe(resource gollm.Resource) string {
	if name := resource.Name(); name != "" && !strings.Contains(name, "://") && name != resource.URI() {
		return name
	}
	mediaType, _, _ := mime.ParseMediaType(resource.Type())
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		return "the image"
	case strings.HasPrefix(mediaType, "audio/"):
		return "the recording"
	case strings.HasPrefix(mediaType, "video/"):
		return "the video"
	case mediaType == "application/pdf":
		return "the PDF"
	}
	return "the document"
}

// voiceOf returns the voice the options set, or an empty string
func voiceOf(opts []gollm.Opt) string {
	o, err := gollm.ApplyOpts(opts...)
	if err != nil {
		return ""
	}
	voice, _ := gollm.GetOpt[string](o, gollm.VoiceKey)
	return voice
}
