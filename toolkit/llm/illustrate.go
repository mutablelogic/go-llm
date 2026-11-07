package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// illustrate is the tool which makes images, with a model, from a prompt,
// or by changing, or combining, images, by their URIs, which are kept, and
// returned as links to them
type illustrate struct {
	toolkit.Base
	models ModelsFunc
}

// illustrateInput is the input of the illustrate tool
type illustrateInput struct {
	Prompt      string   `json:"prompt" help:"What to make, or how to change the images, described in detail: the subject, its setting, the style, and the mood"`
	Images      []string `json:"images,omitempty" help:"URIs of images to change, or combine, which are resource:///<hash>, as attachments, or tools' results, say, or none, to make a new image" optional:""`
	AspectRatio string   `json:"aspect_ratio,omitempty" help:"Aspect ratio of the image, such as 1:1, 16:9 or 9:16, or the model's, when it's omitted" optional:""`
	Model       string   `json:"model,omitempty" help:"Model which makes the image, by its name, such as gemini:gemini-3.1-flash-lite-image" optional:""`
	Name        string   `json:"name,omitempty" help:"Name of the image's file, such as banana, which its format's extension is added to, or image, when it's omitted" optional:""`
	Title       string   `json:"title,omitempty" help:"Title of the image, for people, such as A banana at sunset, which it's shown with" optional:""`
}

// illustrateOutput is how many images were made, which are linked to
type illustrateOutput struct {
	Images int `json:"images"`
}

var _ gollm.Tool = (*illustrate)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*illustrate) Name() string {
	return "illustrate"
}

func (*illustrate) Description() string {
	return withDefault("Makes an image from a description, or changes, or combines, images, which are attachments, or tools' results, by their URIs, resource:///<hash>, as the description says, such as \"put it at sunset\". The image is kept, and returned as a link, which the person sees. Use it when the person asks for a picture, a drawing, or an illustration, or to change one.", "", "image_output")
}

func (*illustrate) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[illustrateInput]()
}

// Timeout returns the most time the tool has, which is longer than a tool
// has by default, for making, or changing, an image
func (*illustrate) Timeout() time.Duration {
	return illustrateTimeout
}

// Run makes the images, with the model, from the prompt, and the images, by
// their URIs, and returns them, named, and titled, as the model says,
// saying what it's doing, and the tokens it used
func (t *illustrate) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req illustrateInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, gollm.ErrBadParameter.With(err.Error())
	} else if req.Prompt = strings.TrimSpace(req.Prompt); req.Prompt == "" {
		return nil, gollm.ErrBadParameter.With("missing prompt")
	}
	models, model, err := modelFor(ctx, t.models, req.Model, "", "image_output")
	if err != nil {
		return nil, err
	}

	// The images to change, by their URIs, which are read together, once
	// each, within a limit
	uris := make([]string, 0, len(req.Images))
	for _, uri := range req.Images {
		uris = append(uris, strings.TrimSpace(uri))
	}
	var images []gollm.Resource
	if len(uris) > 0 {
		if images, err = toolkit.Resources(ctx, uris...); err != nil {
			return nil, err
		}
	}
	var opts []gollm.Opt
	if ratio := strings.TrimSpace(req.AspectRatio); ratio != "" {
		opts = append(opts, gollm.WithOpt(gollm.AspectRatioKey, ratio))
	}

	switch len(images) {
	case 0:
		toolkit.Logf(ctx, "Making an image")
	case 1:
		toolkit.Logf(ctx, "Changing %s", describe(images[0]))
	default:
		toolkit.Logf(ctx, "Combining %d images", len(images))
	}
	made, usage, err := models.Illustrate(ctx, model, req.Prompt, images, opts...)
	if err != nil {
		return nil, err
	}
	toolkit.SessionFromContext(ctx).Usage(usage)

	// The images, named, and titled, as the model says, numbered, when
	// there's more than one
	result := make([]gollm.Resource, 0, len(made))
	for i, image := range made {
		name := fileName(req.Name)
		if name == "" {
			name = "image"
		}
		if len(made) > 1 {
			name = fmt.Sprintf("%s-%d", name, i+1)
		}
		result = append(result, &named{Resource: image, name: name + imageExtension(image.Type()), title: oneLine(req.Title)})
	}
	return toolkit.Result{Value: illustrateOutput{Images: len(result)}, Resources: result}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// imageExtension returns the extension of an image of the type, such as .png
func imageExtension(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])) {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/png":
		return ".png"
	}
	return ""
}
