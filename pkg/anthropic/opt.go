package anthropic

import (
	// Packages

	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type opt struct {
	MaxTokens     uint         `json:"max_tokens,omitempty"`
	Metadata      *optmetadata `json:"metadata,omitempty"`
	StopSequences []string     `json:"stop_sequences,omitempty"`
	Stream        *bool        `json:"stream,omitempty"`
	System        string       `json:"system,omitempty"`
	Temperature   float64      `json:"temperature,omitempty"`
	TopK          uint         `json:"top_k,omitempty"`
	TopP          float64      `json:"top_p,omitempty"`

	data []Attachment
}

type optmetadata struct {
	User string `json:"user_id,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func apply(opts ...llm.Opt) (*opt, error) {
	o := new(opt)
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS
