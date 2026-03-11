package tool

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// namespacedTool wraps an llm.Tool and prepends a namespace to its name,
// delegating all other methods to the underlying tool.
type namespacedTool struct {
	llm.Tool
	name string
}

var _ llm.Tool = (*namespacedTool)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithNamespace returns t with its name replaced by "namespace.t.Name()".
func WithNamespace(namespace string, t llm.Tool) llm.Tool {
	return &namespacedTool{
		Tool: t,
		name: namespace + "." + t.Name(),
	}
}

func (n *namespacedTool) Name() string { return n.name }
