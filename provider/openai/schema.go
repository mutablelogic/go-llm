package openai

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// apiModel is the wire format for a single model, returned by GET /models.
// OpenAI-compatible servers often add fields of their own, such as a context
// length, so every field is also kept in Fields.
// Reference: https://platform.openai.com/docs/api-reference/models/object
type apiModel struct {
	ID      string `json:"id"`
	Object  string `json:"object,omitempty"`
	Created int64  `json:"created,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`

	// All the fields of the model, including those above
	Fields map[string]json.RawMessage `json:"-"`
}

// listModelsResponse is the wire format for GET /models.
// Reference: https://platform.openai.com/docs/api-reference/models/list
type listModelsResponse struct {
	Object string     `json:"object,omitempty"`
	Data   []apiModel `json:"data"`
}

///////////////////////////////////////////////////////////////////////////////
// JSON

// UnmarshalJSON decodes the model's fields, and keeps all of them in Fields
func (m *apiModel) UnmarshalJSON(data []byte) error {
	type model apiModel
	var result model
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &result.Fields); err != nil {
		return err
	}
	*m = apiModel(result)
	return nil
}

// chatRequest is the wire format for POST /chat/completions. Options which
// aren't set are left out, so the server's defaults are used. OpenAI's own
// API takes the most tokens to generate as max_completion_tokens, which its
// reasoning models need, and other servers as max_tokens.
// Reference: https://platform.openai.com/docs/api-reference/chat/create
type chatRequest struct {
	Model               string          `json:"model"`
	Messages            []chatMessage   `json:"messages"`
	Tools               []chatTool      `json:"tools,omitempty"`
	ToolChoice          string          `json:"tool_choice,omitempty"` // "auto", "required" or "none"
	Stream              bool            `json:"stream,omitempty"`
	StreamOptions       *streamOptions  `json:"stream_options,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	MaxTokens           *uint           `json:"max_tokens,omitempty"`
	MaxCompletionTokens *uint           `json:"max_completion_tokens,omitempty"`
	Stop                []string        `json:"stop,omitempty"`
	Seed                *uint           `json:"seed,omitempty"`
	RandomSeed          *uint           `json:"random_seed,omitempty"` // Mistral's seed
	PresencePenalty     *float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty    *float64        `json:"frequency_penalty,omitempty"`
	ResponseFormat      *responseFormat `json:"response_format,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"` // "minimal", "low", "medium" or "high"
	Thinking            *chatThinking   `json:"thinking,omitempty"`         // DeepSeek's thinking mode
	User                string          `json:"user,omitempty"`
	ServiceTier         string          `json:"service_tier,omitempty"`
}

// chatThinking turns DeepSeek's thinking mode on or off
type chatThinking struct {
	Type string `json:"type"` // "enabled" or "disabled"
}

// streamOptions asks for the tokens used in the last chunk of a stream
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// responseFormat is the JSON schema the reply conforms to
type responseFormat struct {
	Type       string          `json:"type"` // Always "json_schema"
	JSONSchema *responseSchema `json:"json_schema,omitempty"`
}

// responseSchema is a named JSON schema
type responseSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
}

// chatMessage is a message in a conversation. Its content is text, or parts
// of text and images, or null for a reply with only tool calls. A reply's
// reasoning is in reasoning_content, for servers which return it, such as
// DeepSeek, which needs it sent back with the reply's tool calls. A reply the
// model refused has why in refusal. A tool's result has the tool role, with
// the call's identifier.
type chatMessage struct {
	Role             string         `json:"role"`
	Content          any            `json:"content"` // string, []chatPart or nil
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	Refusal          string         `json:"refusal,omitempty"`
	ToolCalls        []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
}

// chatPart is a part of a message's content, which is text or an image
type chatPart struct {
	Type     string        `json:"type"` // "text" or "image_url"
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

// chatImageURL is an image, as a URL, which can be a data URL
type chatImageURL struct {
	URL string `json:"url"`
}

// chatTool is a tool which the model can call
type chatTool struct {
	Type     string           `json:"type"` // Always "function"
	Function chatToolFunction `json:"function"`
}

// chatToolFunction describes a tool's function, with its parameters as a
// JSON schema
type chatToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// chatToolCall is a call to a tool, in a message from the model, with its
// arguments as a string of JSON. When streamed, the pieces of each call have
// its index, and only the first has its identifier and name.
type chatToolCall struct {
	Index    *int                 `json:"index,omitempty"`
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type,omitempty"` // Always "function"
	Function chatToolCallFunction `json:"function"`
}

// chatToolCallFunction is the function called, with its arguments
type chatToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

// chatResponse is the wire format for a response from POST /chat/completions,
// and for each chunk of a streamed response, whose choices have a delta
// rather than a message. The last chunk of a stream has the usage, and no
// choices.
// Reference: https://platform.openai.com/docs/api-reference/chat/object
type chatResponse struct {
	ID      string       `json:"id,omitempty"`
	Model   string       `json:"model,omitempty"`
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage,omitempty"`
	Error   *chatError   `json:"error,omitempty"`
}

// chatChoice is a reply, and why the model stopped, which is "stop",
// "length", "tool_calls" or "content_filter"
type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	Delta        chatMessage `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// chatUsage is the tokens used, of which those read from the cache, which
// DeepSeek also returns as prompt_cache_hit_tokens, and of reasoning
type chatUsage struct {
	PromptTokens            uint                    `json:"prompt_tokens"`
	CompletionTokens        uint                    `json:"completion_tokens"`
	PromptTokensDetails     *promptTokensDetails    `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *completionTokenDetails `json:"completion_tokens_details,omitempty"`
	PromptCacheHitTokens    uint                    `json:"prompt_cache_hit_tokens,omitempty"`
}

// promptTokensDetails are the prompt tokens read from the cache
type promptTokensDetails struct {
	CachedTokens uint `json:"cached_tokens"`
}

// completionTokenDetails are the completion tokens of reasoning
type completionTokenDetails struct {
	ReasoningTokens uint `json:"reasoning_tokens"`
}

// chatError is an error, which a server can send in a stream
type chatError struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Usage returns the tokens used, with those read from the cache, and of
// reasoning, from the details or DeepSeek's cache hits
func (u *chatUsage) Usage() llm.Usage {
	if u == nil {
		return llm.Usage{}
	}
	usage := llm.Usage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens, CachedTokens: u.PromptCacheHitTokens}
	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
		usage.CachedTokens = u.PromptTokensDetails.CachedTokens
	}
	if u.CompletionTokensDetails != nil {
		usage.ReasoningTokens = u.CompletionTokensDetails.ReasoningTokens
	}
	return usage
}

///////////////////////////////////////////////////////////////////////////////
// EMBEDDINGS

// embedRequest is a request to embed texts, each as a vector, with the
// number of dimensions of the vectors, which OpenAI's text-embedding-3
// models, and Mistral's codestral-embed, can make fewer than their own, and
// the vectors as numbers rather than base64
// Reference: https://platform.openai.com/docs/api-reference/embeddings
type embedRequest struct {
	Model           string   `json:"model"`
	Input           []string `json:"input"`
	Dimensions      uint     `json:"dimensions,omitempty"`
	OutputDimension uint     `json:"output_dimension,omitempty"` // Mistral's dimensions
	EncodingFormat  string   `json:"encoding_format,omitempty"`
	User            string   `json:"user,omitempty"`
}

// embedResponse is the vector of each text, by its index, and the tokens in
// the texts, or an error, which some servers return with a success status
type embedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens uint `json:"prompt_tokens"`
	} `json:"usage"`
	Error *chatError `json:"error,omitempty"`
}

// ocrRequest is a request to recognize the text in an image or document,
// which is Mistral's, with the pages, from zero, and the format of tables
// Reference: https://docs.mistral.ai/api/endpoint/ocr
type ocrRequest struct {
	Model         string      `json:"model"`
	Document      ocrDocument `json:"document"`
	Pages         []uint      `json:"pages,omitempty"`
	TableFormat   string      `json:"table_format,omitempty"`
	IncludeImages *bool       `json:"include_image_base64,omitempty"`
	IncludeBlocks *bool       `json:"include_blocks,omitempty"` // The regions of each page
}

// ocrDocument is the image or document to recognize, by its URL, which is
// a data URL
type ocrDocument struct {
	Type        string `json:"type"` // image_url or document_url
	ImageURL    string `json:"image_url,omitempty"`
	DocumentURL string `json:"document_url,omitempty"`
}

// ocrResponse is the text of each page, in Markdown, with its size, in
// pixels, and its blocks, which are its regions, with their boxes, in
// pixels, and the pages processed
type ocrResponse struct {
	Pages []struct {
		Index      int    `json:"index"`
		Markdown   string `json:"markdown"`
		Dimensions struct {
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
		} `json:"dimensions"`
		Blocks []struct {
			Type    string  `json:"type"`
			Content string  `json:"content"`
			Left    float64 `json:"top_left_x"`
			Top     float64 `json:"top_left_y"`
			Right   float64 `json:"bottom_right_x"`
			Bottom  float64 `json:"bottom_right_y"`
		} `json:"blocks"`
	} `json:"pages"`
	Model string `json:"model"`
	Usage struct {
		PagesProcessed uint `json:"pages_processed"`
	} `json:"usage_info"`
}
