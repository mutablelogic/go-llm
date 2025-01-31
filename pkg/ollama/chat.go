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

// Chat Response
type Response struct {
	Model     string      `json:"model"`
	CreatedAt time.Time   `json:"created_at"`
	Message   MessageMeta `json:"message"`
	Done      bool        `json:"done"`
	Reason    string      `json:"done_reason,omitempty"`
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
	Messages  []*MessageMeta         `json:"messages"`
	Tools     []ToolFunction         `json:"tools,omitempty"`
	Format    string                 `json:"format,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
	Stream    bool                   `json:"stream"`
	KeepAlive *time.Duration         `json:"keep_alive,omitempty"`
}

func (ollama *Client) Chat(ctx context.Context, prompt llm.Context, opts ...llm.Opt) (*Response, error) {
	// Apply options
	opt, err := apply(opts...)
	if err != nil {
		return nil, err
	}

	// Request
	req, err := client.NewJSONRequest(reqChat{
		Model:     prompt.(*session).model.Name(),
		Messages:  prompt.(*session).seq,
		Tools:     opt.tools(ollama),
		Format:    opt.format,
		Options:   opt.options,
		Stream:    opt.stream,
		KeepAlive: opt.keepalive,
	})
	if err != nil {
		return nil, err
	}

	//  Response
	var response, delta Response
	if err := ollama.DoWithContext(ctx, req, &delta, client.OptPath("chat"), client.OptJsonStreamCallback(func(v any) error {
		if v, ok := v.(*Response); !ok || v == nil {
			return llm.ErrConflict.Withf("Invalid stream response: %v", v)
		} else {
			response.Model = v.Model
			response.CreatedAt = v.CreatedAt
			response.Message.Role = v.Message.Role
			response.Message.Content += v.Message.Content
			if v.Done {
				response.Done = v.Done
				response.Metrics = v.Metrics
				response.Reason = v.Reason
			}
		}

		if opt.chatcallback != nil {
			opt.chatcallback(&response)
		}
		return nil
	})); err != nil {
		return nil, err
	}

	// We return the delta or the response
	if opt.stream {
		return &response, nil
	} else {
		return &delta, nil
	}
}
