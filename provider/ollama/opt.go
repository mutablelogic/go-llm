package ollama

import (
	"encoding/json"
	"strings"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
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

// WithThinking enables thinking for the given request context.
// Supported contexts are "chat", "ask", and "generate".
// Ollama thinking is only supported for chat requests.
func WithThinking(context string, budgetTokens ...uint) opt.Opt {
	context = strings.ToLower(strings.TrimSpace(context))
	var budget uint
	if len(budgetTokens) > 0 {
		budget = budgetTokens[0]
	}

	switch context {
	case "chat":
		return withThinkingBudget(budget)
	case "ask", "generate":
		if budget > 0 {
			return opt.Error(schema.ErrBadParameter.Withf("ollama: thinking budget is only supported in chat context, not %q", context))
		}
		return opt.Error(schema.ErrBadParameter.Withf("ollama: thinking is only supported in chat context, not %q", context))
	default:
		return opt.Error(schema.ErrBadParameter.Withf("ollama: unknown thinking context %q", context))
	}
}

// WithJSONOutput constrains the model to produce JSON conforming to the given schema.
// This option is supported by both /api/generate and /api/chat.
func WithJSONOutput(outputSchema *jsonschema.Schema) opt.Opt {
	if outputSchema == nil {
		return opt.Error(schema.ErrBadParameter.With("schema is required for JSON output"))
	}
	data, err := json.Marshal(outputSchema)
	if err != nil {
		return opt.Error(schema.ErrBadParameter.Withf("failed to serialize JSON schema: %v", err))
	}
	return opt.SetString(opt.JSONSchemaKey, string(data))
}

func withThinkingBudget(budgetTokens uint) opt.Opt {
	switch {
	case budgetTokens == 0:
		return opt.SetBool(opt.ThinkingKey, true)
	case budgetTokens <= 32:
		return opt.SetString(opt.ThinkingKey, "low")
	case budgetTokens <= 128:
		return opt.SetString(opt.ThinkingKey, "medium")
	default:
		return opt.SetString(opt.ThinkingKey, "high")
	}
}

// imageOutputKey is the internal opt key for the image-output flag.
const imageOutputKey = "ollama-image-output"
