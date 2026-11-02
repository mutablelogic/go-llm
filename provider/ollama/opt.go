package ollama

import (
	"encoding/json"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// apply sets the options on the request, or returns an error if any option
// isn't valid or isn't supported by Ollama. Generation options are set in the
// request's options, the system prompt is prepended to the messages, so they
// should be set first, as should the tools, which are removed when the tool
// choice is "none", which is the only tool choice Ollama supports. Thinking
// effort takes precedence over thinking unless thinking is disabled.
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
func (r *chatRequest) apply(opts ...llm.Opt) error {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.TemperatureKey:
			err = setOption[float64](r, o, key, "temperature")
		case llm.TopPKey:
			err = setOption[float64](r, o, key, "top_p")
		case llm.TopKKey:
			err = setOption[uint](r, o, key, "top_k")
		case llm.MaxTokensKey:
			err = setOption[uint](r, o, key, "num_predict")
		case llm.StopSequencesKey:
			err = setOption[[]string](r, o, key, "stop")
		case llm.SeedKey:
			err = setOption[uint](r, o, key, "seed")
		case llm.PresencePenaltyKey:
			err = setOption[float64](r, o, key, "presence_penalty")
		case llm.FrequencyPenaltyKey:
			err = setOption[float64](r, o, key, "frequency_penalty")
		case llm.ContextWindowKey:
			err = setOption[uint](r, o, key, "num_ctx")
		case llm.SystemPromptKey:
			var prompt string
			if prompt, err = llm.GetOpt[string](o, key); err == nil {
				r.Messages = append([]chatMessage{{Role: "system", Content: prompt}}, r.Messages...)
			}
		case llm.ThinkingKey:
			r.Think, err = llm.GetOpt[bool](o, key)
		case llm.ThinkingEffortKey:
			_, err = llm.GetOpt[string](o, key)
		case llm.JSONSchemaKey:
			r.Format, err = llm.GetOpt[json.RawMessage](o, key)
		case llm.ToolChoiceKey:
			// Ollama has no tool choice, but "none" is no tools
			var choice string
			if choice, err = llm.GetOpt[string](o, key); err == nil && choice == "none" {
				r.Tools = nil
			} else if err == nil {
				err = llm.ErrBadParameter.Withf("tool choice %q isn't supported by ollama", choice)
			}
		case llm.KeepAliveKey:
			var duration time.Duration
			if duration, err = llm.GetOpt[time.Duration](o, key); err == nil {
				r.KeepAlive = duration.String()
			}
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by ollama", key)
		}
		if err != nil {
			return err
		}
	}

	// Thinking effort takes precedence, unless thinking is disabled
	if effort, err := llm.GetOpt[string](o, llm.ThinkingEffortKey); err == nil && r.Think != false {
		r.Think = effort
	}

	// Return success
	return nil
}

// apply sets the options on the request to embed, or returns an error if
// any option isn't valid or isn't supported by Ollama. The number of
// dimensions is Ollama's dimensions, which a model which can make fewer
// than its own supports, and the context window is num_ctx. Ollama has no
// task type or title for embeddings, so they're ignored, as a caller can
// set them for any provider.
func (r *embedRequest) apply(opts ...llm.Opt) error {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.DimensionsKey:
			r.Dimensions, err = llm.GetOpt[uint](o, key)
		case llm.ContextWindowKey:
			var size uint
			if size, err = llm.GetOpt[uint](o, key); err == nil {
				r.Options = map[string]any{"num_ctx": size}
			}
		case llm.KeepAliveKey:
			var duration time.Duration
			if duration, err = llm.GetOpt[time.Duration](o, key); err == nil {
				r.KeepAlive = duration.String()
			}
		case llm.TaskTypeKey, llm.TitleKey:
			// Ollama has no task type or title for embeddings
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by ollama for embeddings", key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// setOption sets the option with the key in the request's options, with
// Ollama's name for it
func setOption[T any](r *chatRequest, o *llm.Opts, key, name string) error {
	value, err := llm.GetOpt[T](o, key)
	if err != nil {
		return err
	}
	if r.Options == nil {
		r.Options = make(map[string]any)
	}
	r.Options[name] = value
	return nil
}

// chatTools returns the tools as the tools in a request, with each tool's
// input schema as its parameters, or nil when there are no tools. A tool
// without an input schema has no parameters. Returns an error if a tool is
// nil, or its name isn't valid or unique.
func chatTools(tools []llm.Tool) ([]chatTool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	result := make([]chatTool, 0, len(tools))
	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if tool == nil {
			return nil, llm.ErrBadParameter.With("missing tool")
		} else if name := tool.Name(); !llm.IsToolName(name) {
			return nil, llm.ErrBadParameter.Withf("invalid tool name %q", name)
		} else if names[name] {
			return nil, llm.ErrBadParameter.Withf("duplicate tool %q", name)
		} else {
			names[name] = true
		}
		parameters := json.RawMessage(`{"type":"object","properties":{}}`)
		if schema := tool.InputSchema(); schema != nil {
			data, err := json.Marshal(schema)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("tool %q: %v", tool.Name(), err)
			}
			parameters = data
		}
		result = append(result, chatTool{
			Type: "function",
			Function: chatToolFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  parameters,
			},
		})
	}
	return result, nil
}
