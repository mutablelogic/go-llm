package ollama

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES - Ollama REST API wire format
//
// Reference: https://github.com/ollama/ollama/blob/main/api/types.go

///////////////////////////////////////////////////////////////////////////////
// CHAT — REQUEST

// chatRequest is the wire format for POST /api/chat.
type chatRequest struct {
	Model       string          `json:"model"`
	Messages    []chatMessage   `json:"messages"`
	Stream      *bool           `json:"stream,omitempty"`
	Format      json.RawMessage `json:"format,omitempty"`
	KeepAlive   *chatDuration   `json:"keep_alive,omitempty"`
	Tools       chatTools       `json:"tools,omitempty"`
	Options     map[string]any  `json:"options,omitempty"`
	Think       *chatThinkValue `json:"think,omitempty"`
	Truncate    *bool           `json:"truncate,omitempty"`
	Shift       *bool           `json:"shift,omitempty"`
	Logprobs    bool            `json:"logprobs,omitempty"`
	TopLogprobs int             `json:"top_logprobs,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// CHAT — RESPONSE

// chatResponse is the wire format for a response from POST /api/chat.
type chatResponse struct {
	Model      string      `json:"model"`
	CreatedAt  time.Time   `json:"created_at"`
	Message    chatMessage `json:"message"`
	Done       bool        `json:"done"`
	DoneReason string      `json:"done_reason,omitempty"`
	chatMetrics
}

// chatMetrics contains token usage and timing metrics returned alongside a response.
type chatMetrics struct {
	TotalDuration      time.Duration `json:"total_duration,omitempty"`
	LoadDuration       time.Duration `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration time.Duration `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       time.Duration `json:"eval_duration,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// MESSAGES

// chatMessage is a single turn in a conversation.
type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	Thinking   string         `json:"thinking,omitempty"`
	Images     [][]byte       `json:"images,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// TOOLS

// chatTools is a list of tools available to the model.
type chatTools []chatTool

// chatTool is a single tool available to the model.
type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

// chatToolCall represents a tool invocation returned in an assistant message.
type chatToolCall struct {
	ID       string               `json:"id,omitempty"`
	Function chatToolCallFunction `json:"function"`
}

// chatToolCallFunction describes the function called by the model.
type chatToolCallFunction struct {
	Index     int             `json:"index"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// chatToolFunction describes a callable function tool.
type chatToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// DURATION

// chatDuration wraps time.Duration with Ollama-compatible JSON encoding.
// A negative value encodes as the integer -1 (keep alive indefinitely);
// non-negative values encode as a Go duration string (e.g. "5m30s").
type chatDuration struct {
	time.Duration
}

func (d chatDuration) MarshalJSON() ([]byte, error) {
	if d.Duration < 0 {
		return []byte("-1"), nil
	}
	return []byte(`"` + d.Duration.String() + `"`), nil
}

func (d *chatDuration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	d.Duration = 5 * time.Minute
	switch t := v.(type) {
	case float64:
		if t < 0 {
			d.Duration = time.Duration(math.MaxInt64)
		} else {
			d.Duration = time.Duration(t * float64(time.Second))
		}
	case string:
		var err error
		d.Duration, err = time.ParseDuration(t)
		if err != nil {
			return err
		}
		if d.Duration < 0 {
			d.Duration = time.Duration(math.MaxInt64)
		}
	default:
		return fmt.Errorf("unsupported duration type: %T", v)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// THINK VALUE

// chatThinkValue is a union type for controlling model reasoning effort.
// It may be a boolean or one of the strings "high", "medium", or "low".
type chatThinkValue struct {
	Value any
}

func (t chatThinkValue) MarshalJSON() ([]byte, error) {
	if t.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(t.Value)
}

func (t *chatThinkValue) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		t.Value = b
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s != "high" && s != "medium" && s != "low" {
			return fmt.Errorf("invalid think value: %q (must be \"high\", \"medium\", \"low\", true, or false)", s)
		}
		t.Value = s
		return nil
	}
	return fmt.Errorf("think must be a boolean or string (\"high\", \"medium\", \"low\", true, or false)")
}

///////////////////////////////////////////////////////////////////////////////
// GENERATE — REQUEST

// generateRequest is the wire format for POST /api/generate.
// Unlike /api/chat, this endpoint accepts a single prompt string and does not
// support tools. It does accept images for multimodal models, and can generate
// images via image-generation models.
type generateRequest struct {
	Model     string          `json:"model"`
	Prompt    string          `json:"prompt"`
	Suffix    string          `json:"suffix,omitempty"`
	System    string          `json:"system,omitempty"`
	Template  string          `json:"template,omitempty"`
	Stream    *bool           `json:"stream,omitempty"`
	Raw       bool            `json:"raw,omitempty"`
	Format    json.RawMessage `json:"format,omitempty"`
	KeepAlive *chatDuration   `json:"keep_alive,omitempty"`
	Images    [][]byte        `json:"images,omitempty"`
	Options   map[string]any  `json:"options,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GENERATE — RESPONSE

// generateResponse is the wire format for a response from POST /api/generate.
type generateResponse struct {
	Model      string    `json:"model"`
	CreatedAt  time.Time `json:"created_at"`
	Response   string    `json:"response"`
	Done       bool      `json:"done"`
	DoneReason string    `json:"done_reason,omitempty"`
	// Image contains a base64-encoded image returned by image-generation models
	// (e.g. x/flux2-klein). Ollama uses the singular field name "image".
	Image string `json:"image,omitempty"`
	// Images contains base64-decoded image bytes returned by multimodal models.
	Images [][]byte `json:"images,omitempty"`
	chatMetrics
}

///////////////////////////////////////////////////////////////////////////////
// EMBED — REQUEST / RESPONSE

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
}

///////////////////////////////////////////////////////////////////////////////
// MODEL

// model represents the API response for a model from Ollama
type model struct {
	Name         string       `json:"name"`
	Model        string       `json:"model,omitempty"`
	ModifiedAt   time.Time    `json:"modified_at"`
	Size         int64        `json:"size,omitempty"`
	Digest       string       `json:"digest,omitempty"`
	Details      ModelDetails `json:"details"`
	Capabilities []string     `json:"capabilities,omitempty"`
	File         string       `json:"modelfile,omitempty"`
	Parameters   string       `json:"parameters,omitempty"`
	Template     string       `json:"template,omitempty"`
	Info         ModelInfo    `json:"model_info,omitempty"`
}

// ModelDetails are the details of the model
type ModelDetails struct {
	ParentModel       string   `json:"parent_model,omitempty"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ModelInfo provides additional model parameters
type ModelInfo map[string]any

// listModelsResponse represents the API response for listing models
type listModelsResponse struct {
	Data []model `json:"models"`
}

// PullStatus provides the status of a pull operation in a callback function
type PullStatus struct {
	Status         string `json:"status"`
	DigestName     string `json:"digest,omitempty"`
	TotalBytes     int64  `json:"total,omitempty"`
	CompletedBytes int64  `json:"completed,omitempty"`
}

func (m model) String() string {
	return types.Stringify(m)
}

func (m ModelDetails) String() string {
	return types.Stringify(m)
}
