package ollama

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// CHAT REQUEST BUILDER

// chatRequestFromOpts builds a chatRequest from the session and applied options.
func chatRequestFromOpts(model string, session *schema.Conversation, options opt.Options) (*chatRequest, error) {
	// Reject options that are incompatible with /api/chat
	if options.GetBool(imageOutputKey) {
		return nil, schema.ErrBadParameter.With("WithImageOutput is not supported by /api/chat: use /api/generate for image-generation models")
	}

	// Convert session to Ollama message format
	messages, err := ollamaMessagesFromSession(session)
	if err != nil {
		return nil, err
	}

	request := &chatRequest{
		Model:    model,
		Messages: messages,
	}

	// System prompt — prepend as a system role message
	if systemPrompt := options.GetString(opt.SystemPromptKey); systemPrompt != "" {
		sysMsg := chatMessage{
			Role:    "system",
			Content: systemPrompt,
		}
		request.Messages = append([]chatMessage{sysMsg}, request.Messages...)
	}

	// Model-specific options go in the Options map
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

	// Thinking — accepts bool or "high"/"medium"/"low"
	if options.Has(opt.ThinkingKey) {
		if v := options.GetString(opt.ThinkingKey); v == "high" || v == "medium" || v == "low" {
			request.Think = &chatThinkValue{Value: v}
		} else {
			request.Think = &chatThinkValue{Value: options.GetBool(opt.ThinkingKey)}
		}
	}

	// JSON schema output format
	if schemaJSON := options.GetString(opt.JSONSchemaKey); schemaJSON != "" {
		request.Format = json.RawMessage(schemaJSON)
	}

	// Collect tools from toolkit and individual WithTool opts
	var allTools []llm.Tool
	if v := options.Get(opt.ToolkitKey); v != nil {
		if tk, ok := v.(*tool.Toolkit); ok {
			allTools = append(allTools, tk.ListTools(schema.ToolListRequest{})...)
		}
	}
	if v := options.Get(opt.ToolKey); v != nil {
		if extra, ok := v.([]llm.Tool); ok {
			allTools = append(allTools, extra...)
		}
	}
	if len(allTools) > 0 {
		tools, err := ollamaToolsFromTools(allTools)
		if err != nil {
			return nil, err
		}
		if len(tools) > 0 {
			request.Tools = tools
		}
	}

	return request, nil
}

// ChatRequest builds a chat request from options without sending it.
// Useful for testing and debugging.
func ChatRequest(model string, session *schema.Conversation, opts ...opt.Opt) (any, error) {
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}
	return chatRequestFromOpts(model, session, options)
}
