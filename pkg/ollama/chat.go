package ollama

import (
	"context"
	"encoding/json"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Chat Completion Response
type Response struct {
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Done      bool      `json:"done"`
	Reason    string    `json:"done_reason,omitempty"`
	Message   `json:"message"`
	Metrics
}

// Metrics
type Metrics struct {
	TotalDuration      time.Duration `json:"total_duration,omitempty"`
	LoadDuration       time.Duration `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration time.Duration `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       time.Duration `json:"eval_duration,omitempty"`
}

var _ llm.Completion = (*Response)(nil)

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

type reqChat struct {
	Model     string                 `json:"model"`
	Messages  []*Message             `json:"messages"`
	Tools     []llm.Tool             `json:"tools,omitempty"`
	Format    string                 `json:"format,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
	Stream    bool                   `json:"stream"`
	KeepAlive *time.Duration         `json:"keep_alive,omitempty"`
}

func (ollama *Client) Chat(ctx context.Context, context llm.Context, opts ...llm.Opt) (*Response, error) {
	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Append the system prompt at the beginning
	messages := make([]*Message, 0, len(context.(*session).seq)+1)
	//if system := opt.SystemPrompt(); system != "" {
	//	messages = append(messages, systemPrompt(system))
	//}

	// Always append the first message of each completion
	for _, message := range context.(*session).seq {
		messages = append(messages, message)
	}

	// Request
	req, err := client.NewJSONRequest(reqChat{
		Model:     context.(*session).model.Name(),
		Messages:  messages,
		Tools:     optTools(ollama, opt),
		Format:    optFormat(opt),
		Options:   optOptions(opt),
		Stream:    optStream(ollama, opt),
		KeepAlive: optKeepAlive(opt),
	})
	if err != nil {
		return nil, err
	}

	//  Response
	var response Response
	reqopts := []client.RequestOpt{
		client.OptPath("chat"),
	}
	if optStream(ollama, opt) {
		reqopts = append(reqopts, client.OptJsonStreamCallback(func(v any) error {
			if v, ok := v.(*Response); !ok || v == nil {
				return llm.ErrConflict.Withf("Invalid stream response: %v", v)
			} else if err := streamEvent(&response, v); err != nil {
				return err
			}
			if fn := opt.StreamFn(); fn != nil {
				fn(&response)
			}
			return nil
		}))
	}

	// Response
	if err := ollama.DoWithContext(ctx, req, &response, reqopts...); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func streamEvent(response, delta *Response) error {
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
	return nil
}
