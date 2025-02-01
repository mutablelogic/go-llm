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
	System    string                 `json:"system,omitempty"`
	KeepAlive *time.Duration         `json:"keep_alive,omitempty"`
}

func (ollama *Client) Chat(ctx context.Context, prompt llm.Context, opts ...llm.Opt) (*Response, error) {
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Append the system prompt at the beginning
	seq := make([]*MessageMeta, 0, len(prompt.(*session).seq)+1)
	if system := opt.SystemPrompt(); system != "" {
		seq = append(seq, &MessageMeta{
			Role:    "system",
			Content: opt.SystemPrompt(),
		})
	}
	seq = append(seq, prompt.(*session).seq...)

	// Request
	req, err := client.NewJSONRequest(reqChat{
		Model:     prompt.(*session).model.Name(),
		Messages:  seq,
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

		//Call the chat callback
		if optStream(ollama, opt) {
			if fn := opt.StreamFn(); fn != nil {
				fn(&response)
			}
		}
		return nil
	})); err != nil {
		return nil, err
	}

	// We return the delta or the response
	if optStream(ollama, opt) {
		return &response, nil
	} else {
		return &delta, nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// INTERFACE - CONTEXT CONTENT

func (response Response) Role() string {
	return response.Message.Role
}

func (response Response) Text() string {
	return response.Message.Content
}

func (response Response) ToolCalls() []llm.ToolCall {
	return nil
}
