package prompt

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
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

// Unwrap returns the underlying prompt, stripping the namespace wrapper.
func (n *namespacedPrompt) Unwrap() llm.Prompt { return n.Prompt }

func (n *namespacedPrompt) MarshalJSON() ([]byte, error) {
	type promptJSON struct {
		Name        string           `json:"name"`
		Title       string           `json:"title,omitempty"`
		Description string           `json:"description,omitempty"`
		Arguments   []promptArgument `json:"arguments,omitempty"`
	}
	v := promptJSON{
		Name:        n.name,
		Title:       n.Title(),
		Description: n.Description(),
	}
	if p, ok := n.Prompt.(*prompt); ok {
		v.Arguments = argsFromInput(p.m.Input)
	}
	return json.Marshal(v)
}

func (n *namespacedPrompt) String() string { return types.Stringify(n) }
