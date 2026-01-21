package anthropic

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

////////////////////////////////////////////////////////////////////////////////
// ANTHROPIC OPTIONS

func WithAfterId(id string) opt.Opt {
	return opt.WithString("after_id", id)
}

func WithBeforeId(id string) opt.Opt {
	return opt.WithString("before_id", id)
}

func WithLimit(limit uint) opt.Opt {
	return opt.WithUint("limit", limit)
}

// WithUser sets the metadata.user_id for the request
func WithUser(value string) opt.Opt {
	return opt.WithString("user_id", value)
}

// WithServiceTier sets the service tier for the request ("auto" or "standard_only")
func WithServiceTier(value string) opt.Opt {
	return opt.WithString("service_tier", value)
}

// WithStopSequences sets custom stop sequences for the request (at least one required)
func WithStopSequences(values ...string) opt.Opt {
	if len(values) == 0 {
		return opt.Error(fmt.Errorf("at least one stop sequence is required"))
	}
	return opt.WithString("stop_sequences", values...)
}

// WithStream enables streaming for the request
func WithStream() opt.Opt {
	return opt.WithString("stream", "true")
}

// WithSystemPrompt sets the system prompt for the request
func WithSystemPrompt(value string) opt.Opt {
	return opt.WithString("system", value)
}

// WithCachedSystemPrompt sets the system prompt with caching enabled
func WithCachedSystemPrompt(value string) opt.Opt {
	return opt.WithOpts(
		opt.WithString("system", value),
		opt.WithString("cache_control", "ephemeral"),
	)
}

// WithTemperature sets the temperature for the request (0.0 to 1.0)
func WithTemperature(value float64) opt.Opt {
	if value < 0 || value > 1 {
		return opt.Error(fmt.Errorf("temperature must be between 0.0 and 1.0"))
	}
	return opt.WithFloat64("temperature", value)
}

// WithThinking enables extended thinking with the specified token budget (minimum 1024)
func WithThinking(budgetTokens uint) opt.Opt {
	if budgetTokens < 1024 {
		return opt.Error(fmt.Errorf("thinking budget must be at least 1024 tokens"))
	}
	return opt.WithUint("thinking_budget", budgetTokens)
}

// WithMaxTokens sets the maximum number of tokens to generate (minimum 1)
func WithMaxTokens(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("max_tokens must be at least 1"))
	}
	return opt.WithUint("max_tokens", value)
}

// WithTopK sets the top K sampling parameter (minimum 1)
func WithTopK(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("top_k must be at least 1"))
	}
	return opt.WithUint("top_k", value)
}

// WithTopP sets the nucleus sampling parameter (0.0 to 1.0)
func WithTopP(value float64) opt.Opt {
	if value < 0 || value > 1 {
		return opt.Error(fmt.Errorf("top_p must be between 0.0 and 1.0"))
	}
	return opt.WithFloat64("top_p", value)
}

// WithOutputConfig sets the output configuration ("low", "medium", or "high")
func WithOutputConfig(value string) opt.Opt {
	if value != "low" && value != "medium" && value != "high" {
		return opt.Error(fmt.Errorf("output_config must be 'low', 'medium', or 'high'"))
	}
	return opt.WithString("output_config", value)
}

// WithJSONOutput sets the output format to JSON with the given schema.
// Use jsonschema.For[T](nil) to infer a schema from a Go type.
func WithJSONOutput(schema *jsonschema.Schema) opt.Opt {
	if schema == nil {
		return opt.Error(fmt.Errorf("schema is required for JSON output"))
	}
	// Serialize the schema to JSON for storage
	data, err := json.Marshal(schema)
	if err != nil {
		return opt.Error(fmt.Errorf("failed to serialize JSON schema: %w", err))
	}
	return opt.WithString("json_schema", string(data))
}

// WithToolChoiceAuto lets the model decide whether to use tools
func WithToolChoiceAuto() opt.Opt {
	return opt.WithString("tool_choice", "auto")
}

// WithToolChoiceAny forces the model to use one of the available tools
func WithToolChoiceAny() opt.Opt {
	return opt.WithString("tool_choice", "any")
}

// WithToolChoiceNone prevents the model from using any tools
func WithToolChoiceNone() opt.Opt {
	return opt.WithString("tool_choice", "none")
}

// WithToolChoice forces the model to use a specific tool by name
func WithToolChoice(name string) opt.Opt {
	if name == "" {
		return opt.Error(fmt.Errorf("tool name is required"))
	}
	return opt.WithOpts(
		opt.WithString("tool_choice", "tool"),
		opt.WithString("tool_choice_name", name),
	)
}

// toolDefinition is the JSON structure for a tool
type toolDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	InputSchema *jsonschema.Schema `json:"input_schema"`
}

// WithTool adds a tool definition to the request.
// Multiple calls append additional tools.
func WithTool(name, description string, schema *jsonschema.Schema) opt.Opt {
	if name == "" {
		return opt.Error(fmt.Errorf("tool name is required"))
	}
	if schema == nil {
		return opt.Error(fmt.Errorf("tool schema is required"))
	}
	tool := toolDefinition{
		Name:        name,
		Description: description,
		InputSchema: schema,
	}
	data, err := json.Marshal(tool)
	if err != nil {
		return opt.Error(fmt.Errorf("failed to serialize tool: %w", err))
	}
	return opt.WithString("tools", string(data))
}
