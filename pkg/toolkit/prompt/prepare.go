package prompt

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READ

// Prepare renders the prompt's Go template against the variables in input (a
// JSON object) and returns the resulting string along with any opts derived
// from the prompt's front matter (model, provider, system prompt, etc.).
func (p *prompt) Prepare(_ context.Context, _ json.RawMessage) (string, []opt.Opt, error) {
	return "", nil, llm.ErrNotImplemented
}
