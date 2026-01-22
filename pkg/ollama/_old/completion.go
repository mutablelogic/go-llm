package ollama

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Chat Response
type Response struct {
	Model     string           `json:"model"`
	CreatedAt time.Time        `json:"created_at"`
	Done      bool             `json:"done"`
	Reason    string           `json:"done_reason,omitempty"`
	Response  *string          `json:"response,omitempty"` // For completion
	Message   `json:"message"` // For chat
	Metrics
}

var _ llm.Completion = (*Response)(nil)

// Metrics
type Metrics struct {
	TotalDuration      time.Duration `json:"total_duration,omitempty"`
	LoadDuration       time.Duration `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration time.Duration `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       time.Duration `json:"eval_duration,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r Response) String() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// https://github.com/ollama/ollama/blob/main/api/types.go
type reqCompletion struct {
	// Model name
	Model string `json:"model"`

	// Prompt is the textual prompt to send to the model.
	Prompt string `json:"prompt"`

	// Suffix is the text that comes after the inserted text.
	Suffix string `json:"suffix,omitempty"`

	// System overrides the model's default system message/prompt.
	System string `json:"system,omitempty"`

	// Template overrides the model's default prompt template.
	Template string `json:"template,omitempty"`

	// Stream specifies whether the response is streaming; it is true by default.
	Stream *bool `json:"stream,omitempty"`

	// Raw set to true means that no formatting will be applied to the prompt.
	Raw bool `json:"raw,omitempty"`

	// Format specifies the format to return a response in.
	Format json.RawMessage `json:"format,omitempty"`

	// KeepAlive controls how long the model will stay loaded in memory following
	// this request.
	KeepAlive *time.Duration `json:"keep_alive,omitempty"`

	// Images is an optional list of base64-encoded images accompanying this
	// request, for multimodal models.
	Images []ImageData `json:"images,omitempty"`

	// Options lists model-specific options. For example, temperature can be
	// set through this field, if the model supports it.
	Options map[string]any `json:"options,omitempty"`
}

// Create a completion from a prompt
func (model *model) Completion(ctx context.Context, prompt string, opts ...llm.Opt) (llm.Completion, error) {
	// Apply options - including prompt options
	opt, err := llm.ApplyPromptOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Make images
	images := make([]ImageData, 0, len(opt.Attachments()))
	for _, attachment := range opt.Attachments() {
		if !strings.HasPrefix(attachment.Type(), "image/") {
			return nil, llm.ErrBadParameter.Withf("Attachment is not an image: %v", attachment.Filename())
		}
		images = append(images, attachment.Data())
	}

	// Request
	req, err := client.NewJSONRequest(reqCompletion{
		Model:     model.Name(),
		Prompt:    prompt,
		System:    opt.SystemPrompt(),
		Stream:    optStream(model.Client, opt),
		Format:    json.RawMessage(optFormat(opt)),
		KeepAlive: optKeepAlive(opt),
		Images:    images,
		Options:   optOptions(opt),
	})
	if err != nil {
		return nil, err
	}

	// Make the request
	return model.request(ctx, req, opt.StreamFn(), client.OptPath("generate"))
}

type reqChat struct {
	Model     string           `json:"model"`
	Messages  []llm.Completion `json:"messages"`
	Tools     []llm.Tool       `json:"tools,omitempty"`
	Format    string           `json:"format,omitempty"`
	Options   map[string]any   `json:"options,omitempty"`
	Stream    *bool            `json:"stream"`
	KeepAlive *time.Duration   `json:"keep_alive,omitempty"`
}

// Create a completion from a chat session
func (model *model) Chat(ctx context.Context, completions []llm.Completion, opts ...llm.Opt) (llm.Completion, error) {
	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Create the completions including the system prompt
	messages := make([]llm.Completion, 0, len(completions)+1)
	if system := opt.SystemPrompt(); system != "" {
		messages = append(messages, messagefactory{}.SystemPrompt(system))
	}
	messages = append(messages, completions...)

	// Request
	req, err := client.NewJSONRequest(reqChat{
		Model:     model.Name(),
		Messages:  messages,
		Tools:     optTools(model.Client, opt),
		Format:    optFormat(opt),
		Options:   optOptions(opt),
		Stream:    optStream(model.Client, opt),
		KeepAlive: optKeepAlive(opt),
	})
	if err != nil {
		return nil, err
	}

	// Make the request
	return model.request(ctx, req, opt.StreamFn(), client.OptPath("chat"))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (model *model) request(ctx context.Context, req client.Payload, streamfn func(llm.Completion), opts ...client.RequestOpt) (*Response, error) {
	var delta, response Response
	if streamfn != nil {
		opts = append(opts, client.OptJsonStreamCallback(func(v any) error {
			if v, ok := v.(*Response); !ok || v == nil {
				return llm.ErrConflict.Withf("Invalid stream response: %v", delta)
			} else if err := streamEvent(&response, v); err != nil {
				return err
			}
			if fn := streamfn; fn != nil {
				fn(&response)
			}
			return nil
		}))
	}

	// Response
	if err := model.DoWithContext(ctx, req, &delta, opts...); err != nil {
		return nil, err
	}

	// Return success
	if streamfn != nil {
		return &response, nil
	} else if delta.Response != nil {
		delta.Message = Message{
			RoleContent: RoleContent{
				Role:    "user",
				Content: *delta.Response,
			},
		}
	}
	return &delta, nil
}

func streamEvent(response, delta *Response) error {
	// Completion instead of chat
	if delta.Response != nil {
		delta.Message = Message{
			RoleContent: RoleContent{
				Role:    "user",
				Content: *delta.Response,
			},
		}
	}

	// Update response from the delta
	if delta.Model != "" {
		response.Model = delta.Model
	}
	if !delta.CreatedAt.IsZero() {
		response.CreatedAt = delta.CreatedAt
	}
	if delta.Message.RoleContent.Role != "" {
		response.Message.RoleContent.Role = delta.Message.RoleContent.Role
	}
	if delta.Message.RoleContent.Content != "" {
		response.Message.RoleContent.Content += delta.Message.RoleContent.Content
	}
	if delta.Done {
		response.Done = delta.Done
		response.Metrics = delta.Metrics
		response.Reason = delta.Reason
	}

	// Return success
	return nil
}
