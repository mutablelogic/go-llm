package anthropic

import (
	"io"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type opt struct {
	MaxTokens     uint         `json:"max_tokens,omitempty"`
	Metadata      *optmetadata `json:"metadata,omitempty"`
	StopSequences []string     `json:"stop_sequences,omitempty"`
	Stream        bool         `json:"stream,omitempty"`
	System        string       `json:"system,omitempty"`
	Temperature   float64      `json:"temperature,omitempty"`
	TopK          uint         `json:"top_k,omitempty"`
	TopP          float64      `json:"top_p,omitempty"`

	// Additional message content
	data []*Content
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

// Messages: Stream the response as it is received.
func WithStream() llm.Opt {
	return func(o any) error {
		o.(*opt).Stream = true
		return nil
	}
}

func WithData(r io.Reader, ephemeral, citations bool) llm.Opt {
	return func(o any) error {
		attachment, err := ReadContent(r, ephemeral, citations)
		if err != nil {
			return err
		}
		o.(*opt).data = append(o.(*opt).data, attachment)
		return nil
	}
}

func WithTemperature(v float64) llm.Opt {
	return func(o any) error {
		if v < 0.0 || v > 1.0 {
			return llm.ErrBadParameter.With("temperature must be between 0.0 and 1.0")
		}
		o.(*opt).Temperature = v
		return nil
	}
}

func WithSystem(v string) llm.Opt {
	return func(o any) error {
		o.(*opt).System = v
		return nil
	}
}

func WithMaxTokens(v uint) llm.Opt {
	return func(o any) error {
		o.(*opt).MaxTokens = v
		return nil
	}
}

func WithUser(v string) llm.Opt {
	return func(o any) error {
		o.(*opt).Metadata = &optmetadata{User: v}
		return nil
	}
}

func WithStopSequences(v ...string) llm.Opt {
	return func(o any) error {
		o.(*opt).StopSequences = v
		return nil
	}
}

func WithTopP(v float64) llm.Opt {
	return func(o any) error {
		if v < 0.0 || v > 1.0 {
			return llm.ErrBadParameter.With("top_p must be between 0.0 and 1.0")
		}
		o.(*opt).TopP = v
		return nil
	}
}

func WithTopK(v uint) llm.Opt {
	return func(o any) error {
		o.(*opt).TopK = v
		return nil
	}
}
