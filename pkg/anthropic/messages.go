package anthropic

import (
	"context"
	"encoding/json"

	// Packages
	"github.com/google/jsonschema-go/jsonschema"
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type messagesRequest struct {
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Messages      *schema.Session   `json:"messages"`
	Metadata      *messagesMetadata `json:"metadata,omitempty"`
	Model         string            `json:"model"`
	OutputConfig  string            `json:"output_config,omitempty"`
	OutputFormat  *outputFormat     `json:"output_format,omitempty"`
	ServiceTier   string            `json:"service_tier,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
	Stream        bool              `json:"stream,omitempty"`
	System        any               `json:"system,omitempty"`
	Temperature   *float64          `json:"temperature,omitempty"`
	Thinking      *thinkingConfig   `json:"thinking,omitempty"`
	ToolChoice    *toolChoice       `json:"tool_choice,omitempty"`
	Tools         []json.RawMessage `json:"tools,omitempty"`
	TopK          *uint             `json:"top_k,omitempty"`
	TopP          *float64          `json:"top_p,omitempty"`
}

type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type outputFormat struct {
	Type       string             `json:"type"`
	JSONSchema *jsonschema.Schema `json:"json_schema,omitempty"`
}

type thinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens uint   `json:"budget_tokens,omitempty"`
}

type messagesMetadata struct {
	UserId string `json:"user_id,omitempty"`
}

type textBlockParam struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	CacheControl *cacheControlEphemeral `json:"cache_control,omitempty"`
}

type cacheControlEphemeral struct {
	Type string `json:"type"`
}

type messagesResponse struct {
	Id           string         `json:"id"`
	Model        string         `json:"model"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      schema.Content `json:"content"`
	StopReason   string         `json:"stop_reason"` // "end_turn", "max_tokens", "stop_sequence", "tool_use", "pause_turn", "refusal"
	StopSequence *string        `json:"stop_sequence,omitempty"`
	messagesUsage
}

type messagesUsage struct {
	InputTokens  uint   `json:"input_tokens"`
	OutputTokens uint   `json:"output_tokens"`
	ServiceTier  string `json:"service_tier"` // standard, priority, batch
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultMaxTokens = 1024
)

// Stop reasons returned by the API
const (
	StopReasonEndTurn      = "end_turn"      // Normal completion
	StopReasonMaxTokens    = "max_tokens"    // Response was truncated
	StopReasonStopSequence = "stop_sequence" // Hit a stop sequence
	StopReasonToolUse      = "tool_use"      // Model wants to use a tool
	StopReasonPauseTurn    = "pause_turn"    // Needs continuation
	StopReasonRefusal      = "refusal"       // Model refused to respond
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// MessagesRequest builds a messages request from options without sending it.
// Useful for testing and debugging.
func MessagesRequest(model string, session *schema.Session, opts ...opt.Opt) (any, error) {
	return messagesRequestFromOpts(model, session, opts...)
}

// messagesRequestFromOpts builds a messagesRequest from the given options
func messagesRequestFromOpts(model string, session *schema.Session, opts ...opt.Opt) (*messagesRequest, error) {
	// Apply the options
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Add metadata if user_id is set
	var metadata *messagesMetadata
	if userId := options.GetString("user_id"); userId != "" {
		metadata = &messagesMetadata{UserId: userId}
	}

	// Set system prompt (cached or plain)
	var system any
	if systemPrompt := options.GetString("system"); systemPrompt != "" {
		if cacheControl := options.GetString("cache_control"); cacheControl != "" {
			system = []textBlockParam{{
				Type:         "text",
				Text:         systemPrompt,
				CacheControl: &cacheControlEphemeral{Type: cacheControl},
			}}
		} else {
			system = systemPrompt
		}
	}

	// Get temperature if set
	var temperature *float64
	if options.Has("temperature") {
		v := options.GetFloat64("temperature")
		temperature = &v
	}

	// Get thinking config if set
	var thinking *thinkingConfig
	if options.Has("thinking_budget") {
		thinking = &thinkingConfig{
			Type:         "enabled",
			BudgetTokens: options.GetUint("thinking_budget"),
		}
	}

	// Get max tokens
	maxTokens := defaultMaxTokens
	if options.Has("max_tokens") {
		maxTokens = int(options.GetUint("max_tokens"))
	}

	// Get top_k if set
	var topK *uint
	if options.Has("top_k") {
		v := options.GetUint("top_k")
		topK = &v
	}

	// Get top_p if set
	var topP *float64
	if options.Has("top_p") {
		v := options.GetFloat64("top_p")
		topP = &v
	}

	// Get output format if JSON schema is set
	var outputFmt *outputFormat
	if schemaJSON := options.GetString("json_schema"); schemaJSON != "" {
		var s jsonschema.Schema
		if err := json.Unmarshal([]byte(schemaJSON), &s); err == nil {
			outputFmt = &outputFormat{
				Type:       "json_schema",
				JSONSchema: &s,
			}
		}
	}

	// Get tool choice if set
	var toolCh *toolChoice
	if tc := options.GetString("tool_choice"); tc != "" {
		toolCh = &toolChoice{Type: tc}
		if tc == "tool" {
			toolCh.Name = options.GetString("tool_choice_name")
		}
	}

	// Get tools if set
	var tools []json.RawMessage
	for _, toolJSON := range options.GetStringArray("tools") {
		tools = append(tools, json.RawMessage(toolJSON))
	}

	return &messagesRequest{
		MaxTokens:     maxTokens,
		Messages:      session,
		Metadata:      metadata,
		Model:         model,
		OutputConfig:  options.GetString("output_config"),
		OutputFormat:  outputFmt,
		ServiceTier:   options.GetString("service_tier"),
		StopSequences: options.GetStringArray("stop_sequences"),
		Stream:        options.GetBool("stream"),
		System:        system,
		Temperature:   temperature,
		Thinking:      thinking,
		ToolChoice:    toolCh,
		Tools:         tools,
		TopK:          topK,
		TopP:          topP,
	}, nil
}

// Messages provides the next message in a session, and updates the session with the response
func (anthropic *Client) Messages(ctx context.Context, model string, session *schema.Session, opts ...opt.Opt) (*schema.Message, error) {
	// Build the request
	req, err := messagesRequestFromOpts(model, session, opts...)
	if err != nil {
		return nil, err
	}

	// Create a request
	request, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	// Send the request
	var response messagesResponse
	if err := anthropic.DoWithContext(ctx, request, &response, client.OptPath("messages")); err != nil {
		return nil, err
	}

	// Check for refusal - no message to append
	if response.StopReason == StopReasonRefusal {
		return nil, llm.ErrRefusal
	}

	// Create a message from the response
	message := schema.Message{
		Role:    response.Role,
		Content: response.Content,
	}

	// Append the message to the session
	session.AppendWithOuput(message, response.InputTokens, response.OutputTokens)

	// Return error for stop reasons that need caller attention
	switch response.StopReason {
	case StopReasonMaxTokens:
		return &message, llm.ErrMaxTokens
	case StopReasonPauseTurn:
		return &message, llm.ErrPauseTurn
	}

	// Return success
	return &message, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r messagesResponse) String() string {
	return schema.Stringify(r)
}
