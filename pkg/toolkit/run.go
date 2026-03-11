package toolkit

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

// Run starts all queued connectors and blocks until ctx is cancelled.
// It closes the toolkit and waits for all connectors to finish on return.
func (tk *toolkit) Run(context.Context) error {
	return llm.ErrNotImplemented
}
