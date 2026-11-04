package toolkit

import (
	"context"
	"maps"
	"slices"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	attribute "go.opentelemetry.io/otel/attribute"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// SystemPrompts returns the system prompts of the namespaces of the tools
// with the names, as Tools returns them, to add to the system prompt of a
// model which can call them, in the order of the names, once each, without
// those which are empty, or the same as one before. A namespace named by
// its name and "*", such as "memory:*", or by "*", has its system prompt,
// even when it has no tools. Returns the same errors as Tools.
func (self *Toolkit) SystemPrompts(ctx context.Context, names ...string) (_ []string, err error) {
	_, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.SystemPrompts",
		attribute.StringSlice("arg.names", names),
	)
	defer func() { endSpan(err) }()

	// The names must be of tools, or namespaces
	if _, _, err := self.resolve(names, false); err != nil {
		return nil, err
	}

	// The namespaces of the names, in their order
	var namespaces []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "*" {
			namespaces = append(namespaces, self.namespaceNames()...)
		} else if namespace, ok := strings.CutSuffix(name, schema.ToolSeparator+"*"); ok {
			namespaces = append(namespaces, namespace)
		} else if entry, err := self.lookup(name); err != nil {
			return nil, err
		} else {
			namespaces = append(namespaces, entry.namespace)
		}
	}

	// Their system prompts, once each
	var result []string
	for _, namespace := range namespaces {
		self.mu.RLock()
		ns, exists := self.namespaces[namespace]
		self.mu.RUnlock()
		if !exists {
			continue
		}
		if prompt := strings.TrimSpace(ns.SystemPrompt()); prompt != "" && !slices.Contains(result, prompt) {
			result = append(result, prompt)
		}
	}
	return result, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// namespaceNames returns the names of the namespaces, in order
func (self *Toolkit) namespaceNames() []string {
	self.mu.RLock()
	defer self.mu.RUnlock()
	return slices.Sorted(maps.Keys(self.namespaces))
}
