package schema

import (
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// RecognizeRequest asks a model for the text in an image, or a document
// such as a PDF, by its type and data, with the options
type RecognizeRequest struct {
	Model string                     `json:"model" help:"Model which recognizes text, by name, or by provider and name" example:"mistral:mistral-ocr-latest"`
	Type  string                     `json:"type" help:"MIME type of the image or document" example:"application/pdf"`
	Data  []byte                     `json:"data" help:"Data of the image or document, encoded as base64" example:"JVBERi0xLjQ="`
	Opt   map[string]json.RawMessage `json:"opt,omitzero" help:"Options, by key, such as pages=\"0,2-4\", table_format=html, and regions=true, which the provider decodes" example:"{\"pages\":\"0-1\"}" optional:""`
}

// RecognizeForm is the form for recognizing the text in an image or
// document uploaded as a file, such as from a web browser, which is read as
// a RecognizeRequest. The options are a JSON object, as a form's values are
// strings.
type RecognizeForm struct {
	Model string     `json:"model" help:"Model which recognizes text, by name, or by provider and name" example:"mistral:mistral-ocr-latest"`
	Opt   string     `json:"opt,omitempty" help:"Options, as a JSON object, such as {\"pages\":\"0-1\"}" example:"{\"pages\":\"0-1\"}" optional:""`
	File  types.File `json:"file,omitzero" help:"Image or document to recognize, uploaded as a file"`
}

// RecognizeResponse is the text of each page, in order, in Markdown, and
// its regions, when they're asked for, with the model which recognized it,
// and the pages, and tokens, used
type RecognizeResponse struct {
	Model string `json:"model" help:"Canonical name of the model which recognized the text" example:"mistral:mistral-ocr-latest"`
	llm.Document
	Usage llm.Usage `json:"usage" help:"Pages recognized, and tokens used, for providers which report them"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// RecognizeBodyLimit is the largest request body, in bytes, when text is
// recognized, which allows for a document of 48 MiB encoded as base64
const RecognizeBodyLimit = 64 << 20

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Resource returns the image or document to recognize, or an error when it
// has no type, or no data
func (r RecognizeRequest) Resource() (llm.Resource, error) {
	if strings.TrimSpace(r.Type) == "" {
		return nil, llm.ErrBadParameter.With("missing type")
	} else if len(r.Data) == 0 {
		return nil, llm.ErrBadParameter.With("missing data")
	}
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: strings.TrimSpace(r.Type), Data: r.Data}}, nil
}

// Opts returns the options, for the provider to decode
func (r RecognizeRequest) Opts() []llm.Opt {
	opts := make([]llm.Opt, 0, len(r.Opt))
	for key, value := range r.Opt {
		opts = append(opts, llm.WithOpt(key, value))
	}
	return opts
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// String returns the request without its data, which can be large
func (r RecognizeRequest) String() string {
	r.Data = nil
	return types.Stringify(r)
}

func (r RecognizeResponse) String() string {
	return types.Stringify(r)
}
