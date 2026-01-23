package gemini

import (
	"context"
	"encoding/json"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Request structure for generateContent
type generateContentRequest struct {
	Contents          []contentRequest  `json:"contents"`
	SystemInstruction *contentRequest   `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
	SafetySettings    []safetySetting   `json:"safetySettings,omitempty"`
	Tools             []tool            `json:"tools,omitempty"`
	ToolConfig        *toolConfig       `json:"toolConfig,omitempty"`
}

type contentRequest struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text             string            `json:"text,omitempty"`
	InlineData       *inlineData       `json:"inlineData,omitempty"`
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64-encoded
}

type functionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type functionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type generationConfig struct {
	StopSequences   []string `json:"stopSequences,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	CandidateCount  *int     `json:"candidateCount,omitempty"`
}

type safetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type tool struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations,omitempty"`
}

type functionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type toolConfig struct {
	FunctionCallingConfig *functionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type functionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"` // AUTO, ANY, NONE
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// Response structure for generateContent
type generateContentResponse struct {
	Candidates     []candidate     `json:"candidates"`
	PromptFeedback *promptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *usageMetadata  `json:"usageMetadata,omitempty"`
	ModelVersion   string          `json:"modelVersion,omitempty"`
}

type candidate struct {
	Content       *contentResponse `json:"content,omitempty"`
	FinishReason  string           `json:"finishReason,omitempty"`
	Index         int              `json:"index"`
	SafetyRatings []safetyRating   `json:"safetyRatings,omitempty"`
}

type contentResponse struct {
	Role  string `json:"role"`
	Parts []part `json:"parts"`
}

type safetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
	Blocked     bool   `json:"blocked"`
}

type promptFeedback struct {
	BlockReason   string         `json:"blockReason,omitempty"`
	SafetyRatings []safetyRating `json:"safetyRatings,omitempty"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultMaxTokens = 1024
)

// Finish reasons
const (
	FinishReasonStop      = "STOP"
	FinishReasonMaxTokens = "MAX_TOKENS"
	FinishReasonSafety    = "SAFETY"
	FinishReasonRecite    = "RECITATION"
	FinishReasonOther     = "OTHER"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Chat generates the next message in a conversation with a Google model.
// It updates the session with the response.
func (c *Client) Chat(ctx context.Context, model string, session *schema.Session, opts ...opt.Opt) (*schema.Message, error) {
	// Build the request
	req, err := generateContentRequestFromOpts(session, opts...)
	if err != nil {
		return nil, err
	}

	// Create a JSON request
	request, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	// Send the request
	var response generateContentResponse
	if err := c.DoWithContext(ctx, request, &response, client.OptPath("models", model+":generateContent")); err != nil {
		return nil, err
	}

	// Convert response to schema message
	message := response.toSchemaMessage()

	// Append the message to the session with token counts
	inputTokens := uint(0)
	outputTokens := uint(0)
	if response.UsageMetadata != nil {
		inputTokens = uint(response.UsageMetadata.PromptTokenCount)
		outputTokens = uint(response.UsageMetadata.CandidatesTokenCount)
	}
	session.AppendWithOuput(message, inputTokens, outputTokens)

	// Return success
	return &message, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// generateContentRequestFromOpts builds a request from options
func generateContentRequestFromOpts(session *schema.Session, opts ...opt.Opt) (*generateContentRequest, error) {
	// Apply the options
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Convert session messages to content requests
	contents := sessionToContents(session)

	// Build generation config
	var genConfig *generationConfig
	if options.Has("max_tokens") || options.Has("temperature") || options.Has("top_p") || options.Has("top_k") || options.Has("stop_sequences") {
		genConfig = &generationConfig{}
		if options.Has("max_tokens") {
			v := int(options.GetUint("max_tokens"))
			genConfig.MaxOutputTokens = &v
		}
		if options.Has("temperature") {
			v := options.GetFloat64("temperature")
			genConfig.Temperature = &v
		}
		if options.Has("top_p") {
			v := options.GetFloat64("top_p")
			genConfig.TopP = &v
		}
		if options.Has("top_k") {
			v := int(options.GetUint("top_k"))
			genConfig.TopK = &v
		}
		if stopSeqs := options.GetStringArray("stop_sequences"); len(stopSeqs) > 0 {
			genConfig.StopSequences = stopSeqs
		}
	}

	// Build system instruction
	var systemInstruction *contentRequest
	if systemPrompt := options.GetString("system"); systemPrompt != "" {
		systemInstruction = &contentRequest{
			Parts: []part{{Text: systemPrompt}},
		}
	}

	return &generateContentRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		GenerationConfig:  genConfig,
	}, nil
}

// sessionToContents converts a schema session to Google API contents
func sessionToContents(session *schema.Session) []contentRequest {
	if session == nil {
		return nil
	}

	contents := make([]contentRequest, 0, len(*session))
	for _, msg := range *session {
		content := messageToContent(msg)
		if content != nil {
			contents = append(contents, *content)
		}
	}
	return contents
}

// messageToContent converts a schema message to a Google API content request
func messageToContent(msg *schema.Message) *contentRequest {
	if msg == nil {
		return nil
	}

	// Map roles: Google uses "user" and "model"
	role := msg.Role
	switch role {
	case "assistant":
		role = "model"
	case "system":
		// System messages are handled separately in systemInstruction
		return nil
	}

	// Get text content from the message
	text := msg.Text()
	if text == "" {
		return nil
	}

	return &contentRequest{
		Role:  role,
		Parts: []part{{Text: text}},
	}
}

// toSchemaMessage converts the response to a schema message
func (r generateContentResponse) toSchemaMessage() schema.Message {
	if len(r.Candidates) == 0 {
		return schema.Message{}
	}

	candidate := r.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return schema.Message{}
	}

	// Collect all text parts
	var textParts []string
	for _, p := range candidate.Content.Parts {
		if p.Text != "" {
			textParts = append(textParts, p.Text)
		}
	}

	// Create message with combined text
	text := ""
	if len(textParts) > 0 {
		text = textParts[0]
		for i := 1; i < len(textParts); i++ {
			text += "\n" + textParts[i]
		}
	}

	return schema.StringMessage("assistant", text)
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r generateContentResponse) String() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}
