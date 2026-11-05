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
// those which are empty, or the same as one before. Each namespace's system
// prompt is read once, as it can change, such as one which has the time. A namespace named by
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

	// The namespaces of the names, in their order, and the tools of each
	// which are named, or all of them, which is nil
	var namespaces []string
	tools := make(map[string][]string)
	all := make(map[string]bool)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "*" {
			for _, namespace := range self.namespaceNames() {
				namespaces = append(namespaces, namespace)
				all[namespace] = true
			}
		} else if namespace, ok := strings.CutSuffix(name, schema.ToolSeparator+"*"); ok {
			namespaces = append(namespaces, namespace)
			all[namespace] = true
		} else if entry, err := self.lookup(name); err != nil {
			return nil, err
		} else {
			namespaces = append(namespaces, entry.namespace)
			tools[entry.namespace] = append(tools[entry.namespace], entry.tool.Name())
		}
	}

	// Their system prompts, each read once, as it can change, such as one
	// which has the time, without those which are the same as one before
	var result []string
	read := make(map[string]bool, len(namespaces))
	for _, namespace := range namespaces {
		if read[namespace] {
			continue
		}
		read[namespace] = true
		self.mu.RLock()
		ns, exists := self.namespaces[namespace]
		self.mu.RUnlock()
		if !exists {
			continue
		}
		prompt := ns.SystemPrompt()
		if prompter, ok := ns.(ToolPrompter); ok {
			if all[namespace] {
				prompt = prompter.SystemPromptFor(nil)
			} else {
				prompt = prompter.SystemPromptFor(tools[namespace])
			}
		}
		if prompt = strings.TrimSpace(prompt); prompt != "" && !slices.Contains(result, prompt) {
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
