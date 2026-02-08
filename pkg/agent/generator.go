package agent

import (
	"context"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	"github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	"github.com/mutablelogic/go-llm/pkg/provider/google"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultMaxIterations = 10 // Default guard against infinite tool-calling loops
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession sends a single message and returns the response (stateless)
func (a *agent) WithoutSession(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	// Get the client for this model
	client := a.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.Withf("no client found for model: %s", model.Name)
	}

	// Covert options based on client
	opts, err := convertOptsForClient(opts, client)
	if err != nil {
		return nil, err
	}

	// Check if client implements Generator
	generator, ok := client.(llm.Generator)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("client %q does not support messaging", client.Name())
	}

	// Send the message
	return generator.WithoutSession(ctx, model, message, opts...)
}

// WithSession sends a message within a session and returns the response (stateful)
func (a *agent) WithSession(ctx context.Context, model schema.Model, session *schema.Session, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	// Get the client for this model
	client := a.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.Withf("no client found for model: %s", model.Name)
	}

	// Covert options based on client
	opts, err := convertOptsForClient(opts, client)
	if err != nil {
		return nil, err
	}

	// Check if client implements Generator
	generator, ok := client.(llm.Generator)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("client %q does not support messaging", client.Name())
	}

	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Extract toolkit from options if present
	var tk *tool.Toolkit
	if v, ok := o.Get(opt.ToolkitKey).(*tool.Toolkit); ok && v != nil {
		tk = v
	}

	// Extract max iterations from options, falling back to default
	maxIterations := int(defaultMaxIterations)
	if v := o.GetUint(opt.MaxIterationsKey); v > 0 {
		maxIterations = int(v)
	}

	// Send the message
	resp, err := generator.WithSession(ctx, model, session, message, opts...)
	if err != nil {
		return nil, err
	}

	// Loop while the model requests tool calls
	for i := 0; tk != nil && resp.Result == schema.ResultToolCall && i < maxIterations; i++ {
		calls := resp.ToolCalls()
		if len(calls) == 0 {
			break
		}

		// Execute each tool call and collect result blocks
		results := make([]schema.ContentBlock, 0, len(calls))
		for _, call := range calls {
			if result, err := tk.Run(ctx, call.Name, call.Input); err != nil {
				results = append(results, schema.NewToolError(call.ID, call.Name, err))
			} else {
				results = append(results, schema.NewToolResult(call.ID, call.Name, result))
			}
		}

		// Feed tool results back to the model
		resp, err = generator.WithSession(ctx, model, session, &schema.Message{
			Role:    schema.RoleUser,
			Content: results,
		}, opts...)
		if err != nil {
			return nil, err
		}
	}

	// If the loop ended because we hit the iteration limit, report an error
	if tk != nil && resp.Result == schema.ResultToolCall {
		return nil, llm.ErrInternalServerError.Withf("tool call loop did not resolve after %d iterations", maxIterations)
	}

	// Return success
	return resp, nil
}

///////////////////////////////////////////////////////////////////////////////
// AGENT-LEVEL GENERATION OPTIONS

// WithSystemPrompt sets the system instruction, dispatching to the
// correct provider-specific option at call time.
func WithSystemPrompt(value string) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithSystemPrompt(value)
		case schema.Anthropic:
			return anthropic.WithSystemPrompt(value)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: system prompt not supported", provider))
		}
	})
}

// WithTemperature sets the sampling temperature, dispatching to the
// correct provider-specific option at call time.
func WithTemperature(value float64) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithTemperature(value)
		case schema.Anthropic:
			return anthropic.WithTemperature(value)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: temperature not supported", provider))
		}
	})
}

// WithMaxTokens sets the maximum number of output tokens, dispatching to the
// correct provider-specific option at call time.
func WithMaxTokens(value uint) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithMaxTokens(value)
		case schema.Anthropic:
			return anthropic.WithMaxTokens(value)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: max tokens not supported", provider))
		}
	})
}

// WithTopK sets the top-K sampling parameter, dispatching to the
// correct provider-specific option at call time.
func WithTopK(value uint) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithTopK(value)
		case schema.Anthropic:
			return anthropic.WithTopK(value)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: top_k not supported", provider))
		}
	})
}

// WithTopP sets the nucleus sampling parameter, dispatching to the
// correct provider-specific option at call time.
func WithTopP(value float64) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithTopP(value)
		case schema.Anthropic:
			return anthropic.WithTopP(value)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: top_p not supported", provider))
		}
	})
}

// WithStopSequences sets custom stop sequences, dispatching to the
// correct provider-specific option at call time.
func WithStopSequences(values ...string) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithStopSequences(values...)
		case schema.Anthropic:
			return anthropic.WithStopSequences(values...)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: stop sequences not supported", provider))
		}
	})
}

// WithThinking enables extended thinking/reasoning, dispatching to the
// correct provider-specific option at call time.
func WithThinking() opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithThinking()
		case schema.Anthropic:
			return anthropic.WithThinking(10240)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: thinking not supported", provider))
		}
	})
}

// WithJSONOutput constrains the model to produce JSON conforming to the given
// schema, dispatching to the correct provider-specific option at call time.
func WithJSONOutput(s *jsonschema.Schema) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithJSONOutput(s)
		case schema.Anthropic:
			return anthropic.WithJSONOutput(s)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: JSON output not supported", provider))
		}
	})
}

// WithToolkit attaches a toolkit of callable tools to the generation request.
// This is provider-agnostic — each provider reads the toolkit from the options.
func WithToolkit(tk *tool.Toolkit) opt.Opt {
	return tool.WithToolkit(tk)
}

// WithMaxIterations sets the maximum number of tool-call loop iterations.
// Defaults to 10 if not specified.
func WithMaxIterations(n uint) opt.Opt {
	return opt.SetUint(opt.MaxIterationsKey, n)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// convertOptsForClient applies options once, resolves any deferred client-aware
// options, then re-applies the combined set to produce a flat option slice.
func convertOptsForClient(opts []opt.Opt, client llm.Client) ([]opt.Opt, error) {
	// First pass: apply options to collect any WithClient markers
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Resolve client-aware options by provider name
	resolved, err := opt.ConvertOptsForClient(o, client.Name())
	if err != nil {
		return nil, err
	}

	// Return original opts plus the resolved provider-specific opts
	return append(opts, resolved...), nil
}
