package manager

import (
	"context"
	"encoding/json"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	anthropic "github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	eliza "github.com/mutablelogic/go-llm/pkg/provider/eliza"
	google "github.com/mutablelogic/go-llm/pkg/provider/google"
	mistral "github.com/mutablelogic/go-llm/pkg/provider/mistral"
	ollama "github.com/mutablelogic/go-llm/pkg/provider/ollama"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

type generationContext string

const (
	generationContextAsk  generationContext = "ask"
	generationContextChat generationContext = "chat"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ask processes a message and returns a response, outside of a session context (stateless).
// If fn is non-nil, text chunks are streamed to the callback as they arrive.
func (m *Manager) Ask(ctx context.Context, request schema.AskRequest, user *auth.User, fn opt.StreamFn) (_ *schema.AskResponse, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "Ask",
		attribute.String("req", types.Stringify(request.AskRequestCore)),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Resolve model, generator, and options from the request meta
	provider, model, generator, opts, err := m.generatorFromMeta(ctx, request.GeneratorMeta, user, generationContextAsk)
	if err != nil {
		return nil, err
	}

	// Enable streaming when a callback is provided
	if fn != nil {
		opts = append(opts, opt.WithStream(fn))
	}

	// Build message options from attachments
	var msgOpts []opt.Opt
	for i := range request.Attachments {
		a := request.Attachments[i]
		msgOpts = append(msgOpts, opt.AddAny(opt.ContentBlockKey, schema.ContentBlock{
			Attachment: &a,
		}))
	}

	// Create the user message
	message, err := schema.NewMessage(schema.RoleUser, request.Text, msgOpts...)
	if err != nil {
		return nil, err
	}

	// Send the message
	result, usage, err := generator.WithoutSession(ctx, types.Value(model), message, opts...)
	if err != nil {
		return nil, err
	}

	// Create the response
	response := types.Ptr(schema.AskResponse{
		CompletionResponse: schema.CompletionResponse{
			Role:    result.Role,
			Content: result.Content,
			Result:  result.Result,
		},
		Usage: usage,
	})

	// Fold provider metadata into the usage metadata and include the
	// current trace_id for downstream observability.
	response.Usage = mergeUsageMeta(ctx, response.Usage, provider.Meta, result)

	// Insert the usage into the database if we have usage information
	if response.Usage != nil {
		if _, err := m.CreateUsage(ctx, schema.UsageInsert{
			Type:      schema.UsageTypeAsk,
			User:      user.UUID(),
			Model:     model.Name,
			Provider:  types.Ptr(model.OwnedBy),
			UsageMeta: types.Value(response.Usage),
		}); err != nil {
			return nil, err
		}
	}

	// Return success
	return response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// generatorFromMeta resolves the model and generator client from the given
// GeneratorMeta, and returns provider-specific options derived from the meta
// fields (e.g. system prompt). This is reusable for both Ask and Chat.
func (m *Manager) generatorFromMeta(ctx context.Context, meta schema.GeneratorMeta, user *auth.User, context generationContext) (*schema.Provider, *schema.Model, llm.Generator, []opt.Opt, error) {
	// Get candidate providers for user, or all candidates if no user is provided.
	providers, err := m.providersForUser(ctx, types.Value(meta.Provider), user)
	if err != nil {
		return nil, nil, nil, nil, err
	} else if len(providers) == 0 {
		return nil, nil, nil, nil, schema.ErrNotFound.Withf("no providers found for model: %s", types.Value(meta.Model))
	}

	// Get the model
	models, err := m.modelsByName(ctx, providers, types.Value(meta.Model))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// If the model name matches multiple providers, require the provider to be specified for disambiguation.
	var model *schema.Model
	var provider *schema.Provider
	if len(models) == 0 {
		return nil, nil, nil, nil, schema.ErrNotFound.Withf("model %q not found", types.Value(meta.Model))
	} else if len(models) > 1 {
		return nil, nil, nil, nil, schema.ErrConflict.Withf("multiple models named %q found; specify a provider", types.Value(meta.Model))
	} else {
		model = types.Ptr(models[0])
		for i := range providers {
			if providers[i].Name == model.OwnedBy {
				provider = &providers[i]
				break
			}
		}
	}
	if provider == nil {
		return nil, nil, nil, nil, schema.ErrNotFound.Withf("provider %q not found for model: %s", model.OwnedBy, types.Value(meta.Model))
	}

	// Get the provider-specific model
	client := m.Registry.Get(model.OwnedBy)
	if client == nil {
		return nil, nil, nil, nil, schema.ErrNotFound.Withf("no provider found for model: %s", types.Value(meta.Model))
	}

	// Client needs to be a generator
	generator, ok := client.(llm.Generator)
	if !ok {
		return nil, nil, nil, nil, schema.ErrNotImplemented.Withf("provider %q does not support generation", model.OwnedBy)
	}

	// Build options from meta fields
	var opts []opt.Opt
	if meta.SystemPrompt != nil && *meta.SystemPrompt != "" {
		opts = append(opts, withSystemPrompt(*meta.SystemPrompt))
	}
	if len(meta.Format) > 0 {
		opts = append(opts, withJSONOutput(meta.Format))
	}
	if meta.ThinkingBudget != nil && *meta.ThinkingBudget > 0 {
		opts = append(opts, withThinkingBudget(context, *meta.ThinkingBudget))
	} else if meta.Thinking != nil && *meta.Thinking {
		opts = append(opts, withThinking(context))
	}

	// Convert options for the client
	opts, err = convertOptsForClient(opts, client)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Return the resolved model, generator, and options
	return provider, model, generator, opts, nil
}

// withSystemPrompt dispatches to the correct provider-specific system prompt option.
func withSystemPrompt(value string) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithSystemPrompt(value)
		case schema.Anthropic:
			return anthropic.WithSystemPrompt(value)
		case schema.Mistral:
			return mistral.WithSystemPrompt(value)
		case schema.Ollama:
			return opt.SetString(opt.SystemPromptKey, value)
		default:
			return opt.Error(schema.ErrNotImplemented.Withf("%s: WithSystemPrompt not supported", provider))
		}
	})
}

// withThinking dispatches to the correct provider-specific thinking option (no budget).
func withThinking(context generationContext) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithThinking()
		case schema.Eliza:
			return eliza.WithThinking()
		case schema.Ollama:
			return ollama.WithThinking(string(context))
		default:
			return opt.Error(schema.ErrBadParameter.Withf("%s: WithThinking without budget not supported (use --thinking-budget)", provider))
		}
	})
}

// withThinkingBudget dispatches to the correct provider-specific thinking option with a token budget.
func withThinkingBudget(context generationContext, budgetTokens uint) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithThinkingBudget(budgetTokens)
		case schema.Anthropic:
			return anthropic.WithThinking(budgetTokens)
		case schema.Eliza:
			return opt.Error(schema.ErrBadParameter.Withf("%s: WithThinkingBudget not supported (use thinking without a budget)", provider))
		case schema.Ollama:
			return ollama.WithThinking(string(context), budgetTokens)
		default:
			return opt.Error(schema.ErrNotImplemented.Withf("%s: WithThinking not supported", provider))
		}
	})
}

// withJSONOutput dispatches to the correct provider-specific JSON output option.
func withJSONOutput(data schema.JSONSchema) opt.Opt {
	var s jsonschema.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return opt.Error(schema.ErrBadParameter.Withf("invalid JSON schema: %v", err))
	}
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithJSONOutput(&s)
		case schema.Anthropic:
			return anthropic.WithJSONOutput(&s)
		case schema.Mistral:
			return mistral.WithJSONOutput(&s)
		case schema.Ollama:
			return ollama.WithJSONOutput(&s)
		default:
			return opt.Error(schema.ErrNotImplemented.Withf("%s: WithJSONOutput not supported", provider))
		}
	})
}

// convertOptsForClient applies options once, resolves any deferred client-aware
// options, then re-applies the combined set to produce a flat option slice.
func convertOptsForClient(opts []opt.Opt, client llm.Client) ([]opt.Opt, error) {
	// First pass: apply options to collect any WithClient markers
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Resolve client-aware options by client name
	resolved, err := opt.ConvertOptsForClient(o, client.Name())
	if err != nil {
		return nil, err
	}

	// Return original opts plus the resolved client-specific opts
	return append(opts, resolved...), nil
}
