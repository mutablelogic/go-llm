package gemini

import (
	"context"
	"encoding/json"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	impl "github.com/mutablelogic/go-llm/pkg/internal/impl"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Completion Response
type Response struct {
	Candidates      `json:"candidates"`
	*PromptFeedback `json:"promptFeedback,omitempty"`
	*Metrics        `json:"usageMetadata,omitempty"`
	ModelVersion    string `json:"model_version,omitempty"`
}

// Candidate choices
type Candidates []Candidate

// Candidate Variation
type Candidate struct {
	Index  uint64 `json:"index"`
	Reason string `json:"finishReason,omitempty"`
}

// Metrics
type Metrics struct {
	PromptTokenCount        uint64 `json:"promptTokenCount,omitempty"`
	CachedContentTokenCount uint64 `json:"cachedContentTokenCount,omitempty"`
	CandidatesTokenCount    uint64 `json:"candidatesTokenCount,omitempty"`
	ToolUsePromptTokenCount uint64 `json:"toolUsePromptTokenCount,omitempty"`
	TotalTokenCount         uint64 `json:"totalTokenCount,omitempty"`
}

// Prompt Feedback
type PromptFeedback struct {
	BlockReason   string         `json:"blockReason,omitempty"`
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
}

type SafetyRating struct {
	Category    string `json:"category,omitempty"`
	Probability string `json:"probability,omitempty"`
	Blocked     bool   `json:"blocked,omitempty"`
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

func (c Candidate) String() string {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (m Metrics) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

type reqCompletion struct {
}

// Send a completion request with a single prompt, and return the next completion
func (model *model) Completion(ctx context.Context, prompt string, opts ...llm.Opt) (llm.Completion, error) {
	// Create a user prompt
	message, err := messagefactory{}.UserPrompt(prompt, opts...)
	if err != nil {
		return nil, err
	}

	// Chat completion
	return model.Chat(ctx, []llm.Completion{message}, opts...)
}

// Send a completion request with multiple completions, and return the next completion
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
	req, err := client.NewJSONRequest(reqCompletion{
		Model:            model.Name(),
		MaxTokens:        impl.OptMaxTokens(model, opt),
		FrequencyPenalty: impl.OptFrequencyPenalty(opt),
		PresencePenalty:  impl.OptPresencePenalty(opt),
		ResponseFormat:   impl.OptResponseFormat(opt),
		StopSequences:    impl.OptStopSequences(opt),
		Stream:           impl.OptStream(opt),
		StreamOptions:    impl.OptStreamOptions(opt),
		Temperature:      impl.OptTemperature(opt),
		TopP:             impl.OptTopP(opt),
		Tools:            impl.OptTools(model, opt),
		ToolChoice:       impl.OptToolChoice(opt),
		LogProbs:         impl.OptLogProbs(opt),
		TopLogProbs:      impl.OptTopLogProbs(opt),
		Messages:         messages,
	})
	if err != nil {
		return nil, err
	}

	// Response options
	var response Response
	reqopts := []client.RequestOpt{
		client.OptPath("chat", "completions"),
	}

	// Streaming
	/*
		if impl.OptStream(opt) {
			reqopts = append(reqopts, client.OptTextStreamCallback(func(evt client.TextStreamEvent) error {
				if err := streamEvent(&response, evt); err != nil {
					return err
				}
				if fn := opt.StreamFn(); fn != nil {
					fn(&response)
				}
				return nil
			}))
		}
	*/

	// Response
	if err := model.DoWithContext(ctx, req, &response, reqopts...); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// COMPLETIONS

// Return the number of completions
func (c Completions) Num() int {
	return len(c)
}

// Return message for a specific completion
func (c Completions) Choice(index int) llm.Completion {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message
}

// Return the role of the completion
func (c Completions) Role() string {
	// The role should be the same for all completions, let's use the first one
	if len(c) == 0 {
		return ""
	}
	return c[0].Message.Role()
}

// Return the text content for a specific completion
func (c Completions) Text(index int) string {
	if index < 0 || index >= len(c) {
		return ""
	}
	return c[index].Message.Text(0)
}

// Return attachment content for a specific completion
func (c Completions) Attachment(index int) *llm.Attachment {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message.Attachment(0)
}

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
func (c Completions) ToolCalls(index int) []llm.ToolCall {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message.ToolCalls(0)
}
