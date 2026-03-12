package tool

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE TYPES

type toolAnnotationsJSON struct {
	ReadOnlyHint    bool  `json:"readOnlyHint,omitempty"`
	IdempotentHint  bool  `json:"idempotentHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
	OpenWorldHint   *bool `json:"openWorldHint,omitempty"`
}

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

// Unwrap returns the underlying tool, stripping the namespace wrapper.
func (n *namespacedTool) Unwrap() llm.Tool { return n.Tool }

func (n *namespacedTool) MarshalJSON() ([]byte, error) {
	type toolJSON struct {
		Name         string               `json:"name"`
		Title        string               `json:"title,omitempty"`
		Description  string               `json:"description,omitempty"`
		InputSchema  json.RawMessage      `json:"inputSchema,omitempty"`
		OutputSchema json.RawMessage      `json:"outputSchema,omitempty"`
		Annotations  *toolAnnotationsJSON `json:"annotations,omitempty"`
	}
	meta := n.Meta()
	v := toolJSON{
		Name:        n.name,
		Title:       meta.Title,
		Description: n.Description(),
		Annotations: &toolAnnotationsJSON{
			ReadOnlyHint:    meta.ReadOnlyHint,
			IdempotentHint:  meta.IdempotentHint,
			DestructiveHint: meta.DestructiveHint,
			OpenWorldHint:   meta.OpenWorldHint,
		},
	}
	if inputSchema, err := n.InputSchema(); err != nil {
		return nil, err
	} else if inputSchema != nil {
		raw, err := json.Marshal(inputSchema)
		if err != nil {
			return nil, err
		}
		v.InputSchema = raw
	}
	if outputSchema, err := n.OutputSchema(); err != nil {
		return nil, err
	} else if outputSchema != nil {
		raw, err := json.Marshal(outputSchema)
		if err != nil {
			return nil, err
		}
		v.OutputSchema = raw
	}
	return json.Marshal(v)
}

func (n *namespacedTool) String() string { return types.Stringify(n) }
