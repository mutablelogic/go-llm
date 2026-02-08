package mistral

import (
	"encoding/json"
	"fmt"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

////////////////////////////////////////////////////////////////////////////////
// MISTRAL OPTIONS

// WithTemperature sets the temperature for the request (0.0 to 2.0)
func WithTemperature(value float64) opt.Opt {
	if value < 0 || value > 2 {
		return opt.Error(fmt.Errorf("temperature must be between 0.0 and 2.0"))
	}
	return opt.SetFloat64("temperature", value)
}

// WithTopP sets the nucleus sampling parameter (0.0 to 1.0)
func WithTopP(value float64) opt.Opt {
	if value < 0 || value > 1 {
		return opt.Error(fmt.Errorf("top_p must be between 0.0 and 1.0"))
	}
	return opt.SetFloat64("top_p", value)
}

// WithMaxTokens sets the maximum number of tokens to generate
func WithMaxTokens(value uint) opt.Opt {
	return opt.SetUint("max_tokens", value)
}

// WithStopSequences sets custom stop sequences for the request
func WithStopSequences(values ...string) opt.Opt {
	if len(values) == 0 {
		return opt.Error(fmt.Errorf("at least one stop sequence is required"))
	}
	return opt.AddString("stop_sequences", values...)
}

// WithRandomSeed sets the random seed for reproducible results
func WithRandomSeed(value uint) opt.Opt {
	return opt.SetUint("random_seed", value)
}

// WithPresencePenalty sets the presence penalty (-2.0 to 2.0)
func WithPresencePenalty(value float64) opt.Opt {
	if value < -2 || value > 2 {
		return opt.Error(fmt.Errorf("presence_penalty must be between -2.0 and 2.0"))
	}
	return opt.SetFloat64("presence_penalty", value)
}

// WithFrequencyPenalty sets the frequency penalty (-2.0 to 2.0)
func WithFrequencyPenalty(value float64) opt.Opt {
	if value < -2 || value > 2 {
		return opt.Error(fmt.Errorf("frequency_penalty must be between -2.0 and 2.0"))
	}
	return opt.SetFloat64("frequency_penalty", value)
}

// WithSafePrompt enables safe prompt filtering
func WithSafePrompt() opt.Opt {
	return opt.SetBool("safe_prompt", true)
}

// WithStream enables streaming for the request
func WithStream() opt.Opt {
	return opt.SetBool("stream", true)
}

// WithNumChoices sets the number of completions to generate
func WithNumChoices(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("num_choices must be at least 1"))
	}
	return opt.SetUint("num_choices", value)
}

// WithToolChoiceAuto lets the model decide whether to use tools
func WithToolChoiceAuto() opt.Opt {
	return opt.SetString("tool_choice", "auto")
}

// WithToolChoiceAny forces the model to use one of the available tools
func WithToolChoiceAny() opt.Opt {
	return opt.SetString("tool_choice", "any")
}

// WithToolChoiceNone prevents the model from using any tools
func WithToolChoiceNone() opt.Opt {
	return opt.SetString("tool_choice", "none")
}

// WithTool adds a tool definition to the request using Mistral's function format.
// Multiple calls append additional tools.
func WithTool(def schema.ToolDefinition) opt.Opt {
	if def.Name == "" {
		return opt.Error(fmt.Errorf("tool name is required"))
	}
	if def.InputSchema == nil {
		return opt.Error(fmt.Errorf("tool schema is required"))
	}

	wire := struct {
		Type     string `json:"type"`
		Function struct {
			Name        string      `json:"name"`
			Description string      `json:"description,omitempty"`
			Parameters  interface{} `json:"parameters,omitempty"`
		} `json:"function"`
	}{Type: "function"}
	wire.Function.Name = def.Name
	wire.Function.Description = def.Description
	wire.Function.Parameters = def.InputSchema

	data, err := json.Marshal(wire)
	if err != nil {
		return opt.Error(fmt.Errorf("failed to serialize tool: %w", err))
	}
	return opt.AddString("tools", string(data))
}
