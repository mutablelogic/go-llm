package schema

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// IllustrateRequest is a request for a model to make images from the
// prompt, or by changing, or combining, the images, as the prompt says,
// which are kept in the session, when it's given, or on their own
type IllustrateRequest struct {
	Model   string                     `json:"model" help:"Model which makes images, by name, or by provider and name" example:"gemini:gemini-3.1-flash-lite-image"`
	Prompt  string                     `json:"prompt" help:"What to make, or how to change the images" example:"A banana, sunbathing on a beach"`
	Images  []IllustrateImage          `json:"images,omitempty" help:"Images to change, or combine, as the prompt says" optional:""`
	Session string                     `json:"session,omitempty" format:"uuid" help:"Session the images made are kept in, or none, when they're kept on their own" optional:""`
	Opt     map[string]json.RawMessage `json:"opt,omitzero" help:"Options, by key, such as aspect_ratio=16:9, and image_size=2K, which the provider decodes" example:"{\"aspect_ratio\":\"16:9\"}" optional:""`
}

// IllustrateImage is an image to change, as its data, with its type, or a
// resource, by its URI
type IllustrateImage struct {
	Type string `json:"type,omitempty" help:"MIME type of the image, which is needed with its data" example:"image/png" optional:""`
	Data []byte `json:"data,omitempty" help:"Data of the image, encoded as base64, or its URL" example:"iVBORw0KGgo=" optional:""`
	URL  string `json:"url,omitempty" help:"URI of a resource, which is resource:///<hash>, on its own, whose content is read, rather than the data" example:"resource:///9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" optional:""`
}

// IllustrateResponse is the images the model made, the model, by its
// canonical name, and the usage
type IllustrateResponse struct {
	Model  string         `json:"model" help:"Canonical name of the model which made the images" example:"gemini:gemini-3.1-flash-lite-image"`
	Usage  llm.Usage      `json:"usage" help:"How long it took, and the tokens used"`
	Images []Illustration `json:"images" help:"The images the model made"`
}

// Illustration is an image a model made: a resource, by its URI, which is
// kept, or, when the server doesn't keep resources, its data
type Illustration struct {
	URI  string `json:"uri,omitempty" help:"URI of the resource the image is kept as, which is resource:///<hash>" example:"resource:///9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"`
	Name string `json:"name,omitempty" help:"Name of the image's file" example:"image.png"`
	Type string `json:"type" help:"MIME type of the image" example:"image/png"`
	Size int64  `json:"size,omitempty" help:"Size of the image, in bytes" example:"915187"`
	Data []byte `json:"data,omitempty" help:"Data of the image, encoded as base64, when it isn't kept as a resource"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// IllustrateBodyLimit is the largest request body, in bytes, when images are
// made, which has the images to change
const IllustrateBodyLimit = 64 << 20

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Resources returns the images to change, as resources: their data, with
// their types, or resources on their own, by their URIs. Returns
// ErrBadParameter for an image which isn't one of them.
func (r IllustrateRequest) Resources() ([]llm.Resource, error) {
	result := make([]llm.Resource, 0, len(r.Images))
	for i, image := range r.Images {
		resource, err := attachment(image.Type, image.Data, image.URL)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("image %d: %v", i, err)
		}
		result = append(result, resource)
	}
	return result, nil
}

// Opts returns the options, for the provider to decode
func (r IllustrateRequest) Opts() []llm.Opt {
	opts := make([]llm.Opt, 0, len(r.Opt))
	for key, value := range r.Opt {
		opts = append(opts, llm.WithOpt(key, value))
	}
	return opts
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// String returns the request without the data of its images, which can be
// large
func (r IllustrateRequest) String() string {
	images := make([]IllustrateImage, len(r.Images))
	for i, image := range r.Images {
		image.Data = nil
		images[i] = image
	}
	r.Images = images
	return types.Stringify(r)
}

// String returns the response without the data of its images
func (r IllustrateResponse) String() string {
	images := make([]Illustration, len(r.Images))
	for i, image := range r.Images {
		image.Data = nil
		images[i] = image
	}
	r.Images = images
	return types.Stringify(r)
}
