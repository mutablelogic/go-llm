package ollama

import (
	"strconv"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Pull Model: Allow insecure connections for pulling models.
func WithInsecure() llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("insecure", true)
		return nil
	}
}

// Embeddings: Does not truncate the end of each input to fit within context length. Returns error if context length is exceeded.
func WithTruncate() llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("truncate", true)
		return nil
	}
}

// Embeddings & Chat: Controls how long the model will stay loaded into memory following the request.
func WithKeepAlive(v time.Duration) llm.Opt {
	return func(o *llm.Opts) error {
		if v <= 0 {
			return llm.ErrBadParameter.With("keepalive must be greater than zero")
		}
		o.Set("keepalive", v)
		return nil
	}
}

// Pull Model: Stream the response as it is received.
func WithPullStatus(fn func(*PullStatus)) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("pullstatus", fn)
		return nil
	}
}

// Embeddings & Chat: model-specific options.
func WithOption(key string, value any) llm.Opt {
	return func(o *llm.Opts) error {
		if opts, ok := o.Get("options").(map[string]any); !ok {
			o.Set("options", map[string]any{key: value})
		} else {
			opts[key] = value
		}
		return nil
	}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func optInsecure(opts *llm.Opts) bool {
	return opts.GetBool("insecure")
}

func optTruncate(opts *llm.Opts) *bool {
	if !opts.Has("truncate") {
		return nil
	}
	v := opts.GetBool("truncate")
	return &v
}

func optPullStatus(opts *llm.Opts) func(*PullStatus) {
	if fn, ok := opts.Get("pullstatus").(func(*PullStatus)); ok && fn != nil {
		return fn
	}
	return nil
}

func optTools(agent *Client, opts *llm.Opts) []llm.Tool {
	toolkit := opts.ToolKit()
	if toolkit == nil {
		return nil
	}
	return toolkit.Tools(agent)
}

func optFormat(opts *llm.Opts) string {
	format := strings.ToLower(opts.GetString("format"))
	if format == "" {
		return ""
	}
	if format == "json_format" {
		return strconv.Quote("json")
	}
	return strconv.Quote(format)
}

func optStopSequence(opts *llm.Opts) []string {
	if opts.Has("stop") {
		if stop, ok := opts.Get("stop").([]string); ok {
			return stop
		}
	}
	return nil
}

func optOptions(opts *llm.Opts) map[string]any {
	result := make(map[string]any)
	if o, ok := opts.Get("options").(map[string]any); ok {
		for k, v := range o {
			result[k] = v
		}
	}

	// copy across temperature, top_p and top_k
	if opts.Has("temperature") {
		result["temperature"] = opts.Get("temperature").(float64)
	}
	if opts.Has("top_p") {
		result["top_p"] = opts.GetFloat64("top_p")
	}
	if opts.Has("top_k") {
		result["top_k"] = opts.GetUint64("top_k")
	}
	if opts.Has("stop") {
		result["stop"] = opts.Get("stop").([]string)
	}
	if opts.Has("seed") {
		result["seed"] = opts.GetUint64("seed")
	}
	if opts.Has("presence_penalty") {
		result["presence_penalty"] = opts.GetFloat64("presence_penalty")
	}
	if opts.Has("frequency_penalty") {
		result["frequency_penalty"] = opts.GetFloat64("frequency_penalty")
	}

	// Return result
	return result
}

func optStream(agent *Client, opts *llm.Opts) *bool {
	var stream bool

	// Based on stream function
	if opts.StreamFn() != nil {
		stream = true
	}

	// Streaming only if there is a stream function and no tools
	toolkit := opts.ToolKit()
	if toolkit != nil {
		if tools := toolkit.Tools(agent); len(tools) > 0 {
			stream = false
		}
	}

	// Return the value
	return &stream
}

func optKeepAlive(opts *llm.Opts) *time.Duration {
	if v := opts.GetDuration("keepalive"); v > 0 {
		return &v
	}
	return nil
}
