package eliza

import (
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

// WithThinking enables thinking output for ELIZA sessions.
// When enabled, the accumulated memory from the conversation
// is emitted as a thinking content block in the response.
func WithThinking() opt.Opt {
	return opt.SetBool(opt.ThinkingKey, true)
}
