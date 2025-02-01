package anthropic

import (
	"io"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
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

	data     []*Content      // Additional message content
	callback func(*Response) // Streaming callback
	toolkit  *tool.ToolKit   // Toolkit for tools
}

type optmetadata struct {
	User string `json:"user_id,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS

// Messages: Attach data to the request, which can be cached on the server-side
// and cited the response.
func WithAttachment(r io.Reader, ephemeral, citations bool) llm.Opt {
	return func(o *llm.Opt) error {
		attachment, err := ReadContent(r, ephemeral, citations)
		if err != nil {
			return err
		}
		o.(*opt).data = append(o.(*opt).data, attachment)
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
