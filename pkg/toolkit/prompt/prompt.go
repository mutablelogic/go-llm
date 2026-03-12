package prompt

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// namespacedPrompt wraps an llm.Prompt and prepends a namespace to its name,
// delegating all other methods to the underlying prompt.
type namespacedPrompt struct {
	llm.Prompt
	name string
}

var _ llm.Prompt = (*namespacedPrompt)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithNamespace returns p with its name replaced by "namespace.p.Name()".
func WithNamespace(namespace string, p llm.Prompt) llm.Prompt {
	return &namespacedPrompt{
		Prompt: p,
		name:   namespace + "." + p.Name(),
	}
}

func (n *namespacedPrompt) Name() string { return n.name }
