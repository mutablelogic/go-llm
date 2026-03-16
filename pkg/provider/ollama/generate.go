package ollama

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GENERATE REQUEST BUILDER

// generateRequestFromOpts builds a generateRequest from a single message and
// applied options. Unlike chatRequestFromOpts, this targets POST /api/generate
// which takes a prompt string rather than a messages array and does not support
// tools.
func generateRequestFromOpts(model string, message *schema.Message, options opt.Options) (*generateRequest, error) {
	// Reject options that are incompatible with /api/generate
	if options.Has(opt.ToolkitKey) || options.Has(opt.ToolKey) {
		return nil, llm.ErrBadParameter.With("/api/generate does not support tools: use /api/chat instead")
	}
	if options.Has(opt.ToolChoiceKey) {
		return nil, llm.ErrBadParameter.With("/api/generate does not support tool_choice: use /api/chat instead")
	}
	if options.Has(opt.ThinkingKey) {
		return nil, llm.ErrBadParameter.With("/api/generate does not support thinking: use /api/chat instead")
	}

	prompt := generatePromptFromMessage(message)

	images, err := generateImagesFromMessage(message)
	if err != nil {
		return nil, err
	}

	request := &generateRequest{
		Model:  model,
		Prompt: prompt,
		Images: images,
	}

	// System prompt
	request.System = options.GetString(opt.SystemPromptKey)

	// Model-specific options
	var modelOpts map[string]any

	// Temperature
	if options.Has(opt.TemperatureKey) {
		if modelOpts == nil {
			modelOpts = make(map[string]any)
		}
		modelOpts["temperature"] = options.GetFloat64(opt.TemperatureKey)
	}

	// Top P
	if options.Has(opt.TopPKey) {
		if modelOpts == nil {
			modelOpts = make(map[string]any)
		}
		modelOpts["top_p"] = options.GetFloat64(opt.TopPKey)
	}

	// Top K
	if options.Has(opt.TopKKey) {
		if modelOpts == nil {
			modelOpts = make(map[string]any)
		}
		modelOpts["top_k"] = options.GetUint(opt.TopKKey)
	}

	// Max tokens (Ollama calls this num_predict)
	if options.Has(opt.MaxTokensKey) {
		if modelOpts == nil {
			modelOpts = make(map[string]any)
		}
		modelOpts["num_predict"] = options.GetUint(opt.MaxTokensKey)
	}

	// Stop sequences
	if ss := options.GetStringArray(opt.StopSequencesKey); len(ss) > 0 {
		if modelOpts == nil {
			modelOpts = make(map[string]any)
		}
		modelOpts["stop"] = ss
	}

	// Seed
	if options.Has(opt.SeedKey) {
		if modelOpts == nil {
			modelOpts = make(map[string]any)
		}
		modelOpts["seed"] = options.GetUint(opt.SeedKey)
	}

	// Presence penalty
	if options.Has(opt.PresencePenaltyKey) {
		if modelOpts == nil {
			modelOpts = make(map[string]any)
		}
		modelOpts["presence_penalty"] = options.GetFloat64(opt.PresencePenaltyKey)
	}

	// Frequency penalty
	if options.Has(opt.FrequencyPenaltyKey) {
		if modelOpts == nil {
			modelOpts = make(map[string]any)
		}
		modelOpts["frequency_penalty"] = options.GetFloat64(opt.FrequencyPenaltyKey)
	}

	request.Options = modelOpts

	// JSON schema output format
	if schemaJSON := options.GetString(opt.JSONSchemaKey); schemaJSON != "" {
		request.Format = json.RawMessage(schemaJSON)
	}

	return request, nil
}

// GenerateRequest builds a generate request from options without sending it.
// Useful for testing and debugging.
func GenerateRequest(model string, message *schema.Message, opts ...opt.Opt) (any, error) {
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}
	return generateRequestFromOpts(model, message, options)
}
