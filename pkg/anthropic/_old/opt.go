package anthropic

import (
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type optmetadata struct {
	User string `json:"user_id,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func WithEphemeral() llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("ephemeral", true)
		return nil
	}
}

func WithCitations() llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("citations", true)
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func optCitations(opt *llm.Opts) bool {
	return opt.GetBool("citations")
}

func optEphemeral(opt *llm.Opts) bool {
	return opt.GetBool("ephemeral")
}

func optMetadata(opt *llm.Opts) *optmetadata {
	if user, ok := opt.Get("user").(string); ok {
		return &optmetadata{User: user}
	}
	return nil
}

func optTools(agent *Client, opts *llm.Opts) []llm.Tool {
	toolkit := opts.ToolKit()
	if toolkit == nil {
		return nil
	}
	return toolkit.Tools(agent.Name())
}

func optToolChoice(opts *llm.Opts) any {
	choices, ok := opts.Get("tool_choice").([]string)
	if !ok || len(choices) == 0 {
		return nil
	}

	// We only support one choice
	var result struct {
		Type                   string `json:"type"`
		Name                   string `json:"name,omitempty"`
		DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
	}
	choice := strings.TrimSpace(strings.ToLower(choices[0]))
	switch choice {
	case "":
		return nil
	case "auto", "any":
		result.Type = choice
	default:
		result.Type = "tool"
		result.Name = choice
	}
	return result
}

func optMaxTokens(model llm.Model, opt *llm.Opts) uint64 {
	if opt.Has("max_tokens") {
		return opt.GetUint64("max_tokens")
	}
	// https://docs.anthropic.com/en/docs/about-claude/models
	switch {
	case strings.HasPrefix(model.Name(), "claude-3-5"):
		return 8192
	default:
		return 4096
	}
}

func optStopSequences(opt *llm.Opts) []string {
	if opt.Has("stop") {
		if stop, ok := opt.Get("stop").([]string); ok {
			return stop
		}
	}
	return nil
}

func optStream(opt *llm.Opts) bool {
	return opt.StreamFn() != nil
}

func optSystemPrompt(opt *llm.Opts) string {
	return opt.SystemPrompt()
}

func optTemperature(opt *llm.Opts) float64 {
	return opt.GetFloat64("temperature")
}

func optTopK(opt *llm.Opts) uint64 {
	return opt.GetUint64("top_k")
}

func optTopP(opt *llm.Opts) float64 {
	return opt.GetFloat64("top_p")
}
