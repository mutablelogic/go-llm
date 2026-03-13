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

// Prepare returns the prompt for execution, given the input arguments as JSON.
func (p *prompt) Prepare(ctx context.Context, input json.RawMessage) (string, []opt.Opt, error) {
	return "", nil, llm.ErrNotImplemented.With("prompt execution not implemented")
}
