package ollama

import (
	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// OLLAMA-SPECIFIC GENERATION OPTIONS

// WithImageOutput signals that the request targets an image-generation model
// and the response is expected to contain image data rather than text.
// This option is only valid for POST /api/generate; it is rejected by the
// chat endpoint builder.
func WithImageOutput() opt.Opt {
	return opt.SetBool(imageOutputKey, true)
}

// imageOutputKey is the internal opt key for the image-output flag.
const imageOutputKey = "ollama-image-output"
