package ollama

import (
	"encoding/json"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// apiModel is the wire format for a single model, returned by both
// GET /api/tags (list) and POST /api/show (one model) - Ollama uses the same
// shape for both, just with different fields populated.
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
type apiModel struct {
	Name         string                     `json:"name"`
	Model        string                     `json:"model,omitempty"`
	ModifiedAt   time.Time                  `json:"modified_at"`
	Size         int64                      `json:"size,omitempty"`
	Digest       string                     `json:"digest,omitempty"`
	Details      apiModelDetails            `json:"details"`
	Capabilities []string                   `json:"capabilities,omitempty"`
	Info         map[string]json.RawMessage `json:"model_info,omitempty"`
}

// apiModelDetails is the "details" object within apiModel.
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
type apiModelDetails struct {
	ParentModel       string   `json:"parent_model,omitempty"`
	Format            string   `json:"format,omitempty"`
	Family            string   `json:"family,omitempty"`
	Families          []string `json:"families,omitempty"`
	ParameterSize     string   `json:"parameter_size,omitempty"`
	QuantizationLevel string   `json:"quantization_level,omitempty"`
}

// listModelsResponse is the wire format for GET /api/tags.
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
type listModelsResponse struct {
	Models []apiModel `json:"models"`
}

// pullStatus is the status of a pull, from POST /api/pull, which is streamed
// as it progresses.
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
type pullStatus struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
	Error     string `json:"error,omitempty"`
}

// chatRequest is the wire format for POST /api/chat. The options include
// num_ctx, the size of the context window, and generation options such as
// temperature and num_predict.
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
type chatRequest struct {
	Model     string          `json:"model"`
	Messages  []chatMessage   `json:"messages"`
	Stream    *bool           `json:"stream,omitempty"`
	Format    json.RawMessage `json:"format,omitempty"`
	KeepAlive string          `json:"keep_alive,omitempty"` // Duration such as "5m", or negative to keep the model loaded
	Tools     []chatTool      `json:"tools,omitempty"`
	Options   map[string]any  `json:"options,omitempty"`
	Think     any             `json:"think,omitempty"` // true, false, or "high", "medium" or "low"
}

// chatResponse is the wire format for a response from POST /api/chat. When
// streamed, each response has part of the message, and the last has done set,
// with the reason and metrics.
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
type chatResponse struct {
	Model      string      `json:"model"`
	CreatedAt  time.Time   `json:"created_at"`
	Message    chatMessage `json:"message"`
	Done       bool        `json:"done"`
	DoneReason string      `json:"done_reason,omitempty"`
	chatMetrics
}

// chatMetrics are the token counts and timings of a response, with durations
// in nanoseconds
type chatMetrics struct {
	TotalDuration      time.Duration `json:"total_duration,omitempty"`
	LoadDuration       time.Duration `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration time.Duration `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       time.Duration `json:"eval_duration,omitempty"`
}

// chatMessage is a message in a conversation, with images base64-encoded.
// A tool's result has the tool role, with the name of the tool.
type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	Thinking   string         `json:"thinking,omitempty"`
	Images     [][]byte       `json:"images,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
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

// chatToolCall is a call to a tool, in a message from the model
type chatToolCall struct {
	ID       string               `json:"id,omitempty"`
	Function chatToolCallFunction `json:"function"`
}

// chatToolCallFunction is the function called, with its arguments as a JSON
// object
type chatToolCallFunction struct {
	Index     int             `json:"index"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

///////////////////////////////////////////////////////////////////////////////
// EMBED

// embedRequest is a request to embed inputs, each as a vector, which are
// texts, or embedInputs with media, with the number of dimensions of the
// vectors, when the model can make fewer than its own, and options such as
// num_ctx, the size of the context window
// Reference: https://github.com/ollama/ollama/blob/main/docs/openapi.yaml
type embedRequest struct {
	Model      string         `json:"model"`
	Input      []any          `json:"input"`
	Dimensions uint           `json:"dimensions,omitempty"`
	Options    map[string]any `json:"options,omitempty"`
	KeepAlive  string         `json:"keep_alive,omitempty"` // Duration such as "5m", or negative to keep the model loaded
}

// embedInput is an input to embed with media: its text, which can be empty,
// and an image and audio, which are base64 in JSON, for a model with those
// modalities. Ollama embeds them on its MLX runner.
type embedInput struct {
	Text  string `json:"text,omitempty"`
	Image []byte `json:"image,omitempty"`
	Audio []byte `json:"audio,omitempty"`
}

// embedResponse is the vector of each input, in order, and the tokens in the
// inputs
type embedResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float32 `json:"embeddings"`
	PromptEvalCount int         `json:"prompt_eval_count,omitempty"`
}
