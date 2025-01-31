package ollama

import (
	"io"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type opt struct {
	format       string
	stream       bool
	pullcallback func(*PullStatus)
	chatcallback func(*Response)
	insecure     bool
	truncate     *bool
	keepalive    *time.Duration
	options      map[string]any
	data         []Data
	toolkit      *tool.ToolKit // Toolkit for tools
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func apply(opts ...llm.Opt) (*opt, error) {
	o := new(opt)
	o.options = make(map[string]any)
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (o *opt) tools(agent llm.Agent) []ToolFunction {
	if o.toolkit == nil {
		return nil
	}
	var result []ToolFunction
	for _, t := range o.toolkit.Tools(agent) {
		result = append(result, ToolFunction{Type: "function", Function: t})
	}
	return result
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS

// Pull Model: Allow insecure connections for pulling models.
func WithInsecure() llm.Opt {
	return func(o any) error {
		o.(*opt).insecure = true
		return nil
	}
}

// Embeddings: Does not truncate the end of each input to fit within context length. Returns error if context length is exceeded.
func WithTruncate(v bool) llm.Opt {
	return func(o any) error {
		o.(*opt).truncate = &v
		return nil
	}
}

// Embeddings & Chat: Controls how long the model will stay loaded into memory following the request.
func WithKeepAlive(v time.Duration) llm.Opt {
	return func(o any) error {
		o.(*opt).keepalive = &v
		return nil
	}
}

// Pull Model: Stream the response as it is received.
func WithPullStatus(fn func(*PullStatus)) llm.Opt {
	return func(o any) error {
		if fn == nil {
			o.(*opt).stream = false
			o.(*opt).pullcallback = nil
		} else {
			o.(*opt).stream = true
			o.(*opt).pullcallback = fn
		}
		return nil
	}
}

// Chat: Stream the response as it is received.
func WithStream(fn func(*Response)) llm.Opt {
	return func(o any) error {
		if fn == nil {
			return llm.ErrBadParameter.With("callback required")
		}
		o.(*opt).stream = true
		o.(*opt).chatcallback = fn
		return nil
	}
}

// Chat: Append a toolkit to the request
func WithToolKit(v *tool.ToolKit) llm.Opt {
	return func(o any) error {
		if v != nil {
			o.(*opt).toolkit = v
		}
		return nil
	}
}

// Embeddings & Chat: model-specific options.
func WithOption(key string, value any) llm.Opt {
	return func(o any) error {
		if value != nil && key != "" {
			o.(*opt).options[key] = value
		}
		return nil
	}
}

// Chat: attach data.
func WithData(r io.Reader) llm.Opt {
	return func(o any) error {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		o.(*opt).data = append(o.(*opt).data, data)
		return nil
	}
}
