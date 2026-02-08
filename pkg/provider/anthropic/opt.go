package anthropic

import (
	"encoding/json"
	"fmt"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

////////////////////////////////////////////////////////////////////////////////
// PAGINATION OPTIONS

// WithAfterId sets the cursor for forward pagination
func WithAfterId(id string) opt.Opt {
	return opt.SetString(opt.AfterIdKey, id)
}

// WithBeforeId sets the cursor for backward pagination
func WithBeforeId(id string) opt.Opt {
	return opt.SetString(opt.BeforeIdKey, id)
}

// WithLimit sets the page size for pagination
func WithLimit(limit uint) opt.Opt {
	return opt.SetUint(opt.LimitKey, limit)
}

////////////////////////////////////////////////////////////////////////////////
// MESSAGE OPTIONS

// WithUser sets the metadata.user_id for the request
func WithUser(value string) opt.Opt {
	return opt.SetString(opt.UserIdKey, value)
}

// WithServiceTier sets the service tier for the request ("auto" or "standard_only")
func WithServiceTier(value string) opt.Opt {
	return opt.SetString(opt.ServiceTierKey, value)
}

// WithStopSequences sets custom stop sequences for the request
func WithStopSequences(values ...string) opt.Opt {
	if len(values) == 0 {
		return opt.Error(fmt.Errorf("at least one stop sequence is required"))
	}
	return opt.AddString(opt.StopSequencesKey, values...)
}

// WithSystemPrompt sets the system prompt for the request
func WithSystemPrompt(value string) opt.Opt {
	return opt.SetString(opt.SystemPromptKey, value)
}

// WithCachedSystemPrompt sets the system prompt with caching enabled
func WithCachedSystemPrompt(value string) opt.Opt {
	return opt.WithOpts(
		opt.SetString(opt.SystemPromptKey, value),
		opt.SetString(opt.CacheControlKey, "ephemeral"),
	)
}

// WithTemperature sets the temperature for the request (0.0 to 1.0)
func WithTemperature(value float64) opt.Opt {
	if value < 0 || value > 1 {
		return opt.Error(fmt.Errorf("temperature must be between 0.0 and 1.0"))
	}
	return opt.SetFloat64(opt.TemperatureKey, value)
}

// WithThinking enables extended thinking with the specified token budget (minimum 1024)
func WithThinking(budgetTokens uint) opt.Opt {
	if budgetTokens < 1024 {
		return opt.Error(fmt.Errorf("thinking budget must be at least 1024 tokens"))
	}
	return opt.SetUint(opt.ThinkingBudgetKey, budgetTokens)
}

// WithMaxTokens sets the maximum number of tokens to generate (minimum 1)
func WithMaxTokens(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("max_tokens must be at least 1"))
	}
	return opt.SetUint(opt.MaxTokensKey, value)
}

// WithTopK sets the top K sampling parameter (minimum 1)
func WithTopK(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("top_k must be at least 1"))
	}
	return opt.SetUint(opt.TopKKey, value)
}

// WithTopP sets the nucleus sampling parameter (0.0 to 1.0)
func WithTopP(value float64) opt.Opt {
	if value < 0 || value > 1 {
		return opt.Error(fmt.Errorf("top_p must be between 0.0 and 1.0"))
	}
	return opt.SetFloat64(opt.TopPKey, value)
}

// WithOutputConfig sets the output effort ("low", "medium", or "high")
func WithOutputConfig(value string) opt.Opt {
	if value != "low" && value != "medium" && value != "high" {
		return opt.Error(fmt.Errorf("output_config must be 'low', 'medium', or 'high'"))
	}
	return opt.SetString(opt.OutputConfigKey, value)
}

// WithJSONOutput sets the output format to JSON with the given schema
func WithJSONOutput(schema *jsonschema.Schema) opt.Opt {
	if schema == nil {
		return opt.Error(fmt.Errorf("schema is required for JSON output"))
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return opt.Error(fmt.Errorf("failed to serialize JSON schema: %w", err))
	}
	return opt.SetString(opt.JSONSchemaKey, string(data))
}

////////////////////////////////////////////////////////////////////////////////
// TOOL CHOICE OPTIONS

// WithToolChoiceAuto lets the model decide whether to use tools
func WithToolChoiceAuto() opt.Opt {
	return opt.SetString(opt.ToolChoiceKey, "auto")
}

// WithToolChoiceAny forces the model to use one of the available tools
func WithToolChoiceAny() opt.Opt {
	return opt.SetString(opt.ToolChoiceKey, "any")
}

// WithToolChoiceNone prevents the model from using any tools
func WithToolChoiceNone() opt.Opt {
	return opt.SetString(opt.ToolChoiceKey, "none")
}

// WithToolChoice forces the model to use a specific tool by name
func WithToolChoice(name string) opt.Opt {
	if name == "" {
		return opt.Error(fmt.Errorf("tool name is required"))
	}
	return opt.WithOpts(
		opt.SetString(opt.ToolChoiceKey, "tool"),
		opt.SetString(opt.ToolChoiceNameKey, name),
	)
}

// WithTool adds a tool definition to the request.
// Multiple calls append additional tools.
func WithTool(def schema.ToolDefinition) opt.Opt {
	if def.Name == "" {
		return opt.Error(fmt.Errorf("tool name is required"))
	}
	if def.InputSchema == nil {
		return opt.Error(fmt.Errorf("tool schema is required"))
	}
	data, err := json.Marshal(def)
	if err != nil {
		return opt.Error(fmt.Errorf("failed to serialize tool: %w", err))
	}
	return opt.AddString(opt.ToolsKey, string(data))
}
