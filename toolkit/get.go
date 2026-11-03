package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// GetTool returns the tool with the name, which is its canonical name, such
// as "weatherapi:current", or its name without a namespace, such as
// "current". Returns ErrNotFound if there is no tool with the name, or
// ErrConflict if the name has no namespace, and more than one namespace has a
// tool with the name.
func (self *Toolkit) GetTool(ctx context.Context, name string) (_ *schema.Tool, err error) {
	_, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.GetTool",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	// Get the tool
	entry, err := self.lookup(name)
	if err != nil {
		return nil, err
	}

	// Return the tool
	return entry.wire(), nil
}

// Tools returns the tools with the names, which are their canonical names,
// their names without a namespace, or a namespace and "*", such as
// "ollama:*", for all the tools in the namespace. Each tool is wrapped with a
// new name, which is its namespace and name separated by an underscore, such
// as "weatherapi_current". Returns ErrNotFound if there is no tool with a
// name, or no namespace for "*", ErrConflict if a name without a namespace is
// ambiguous, or two tools have the same new name, or ErrBadParameter if a
// tool is named more than once, including by "*", or its new name isn't a
// valid tool name, such as when it's too long.
func (self *Toolkit) Tools(ctx context.Context, names ...string) (_ []llm.Tool, err error) {
	_, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.Tools",
		attribute.StringSlice("arg.names", names),
	)
	defer func() { endSpan(err) }()

	result, _, err := self.resolve(names, false)
	return result, err
}

// AvailableTools returns the tools with the names, as with Tools, except a
// name which isn't a tool, or a namespace which doesn't exist, is skipped,
// rather than an error, as when the tools of a namespace aren't available.
// Returns the names which were skipped, and the same errors as Tools
// otherwise.
func (self *Toolkit) AvailableTools(ctx context.Context, names ...string) (_ []llm.Tool, skipped []string, err error) {
	_, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.AvailableTools",
		attribute.StringSlice("arg.names", names),
	)
	defer func() { endSpan(err) }()

	return self.resolve(names, true)
}

// resolve returns the tools with the names, as with Tools, and when skip is
// true, skips a name which isn't found, and returns the names skipped
func (self *Toolkit) resolve(names []string, skip bool) ([]llm.Tool, []string, error) {
	result := make([]llm.Tool, 0, len(names))
	seen := make(map[string]string, len(names))
	var skipped []string
	for _, name := range names {
		entries, err := self.lookupAll(strings.TrimSpace(name))
		if skip && errors.Is(err, llm.ErrNotFound) {
			skipped = append(skipped, name)
			continue
		} else if err != nil {
			return nil, nil, err
		}
		for _, entry := range entries {
			canonical := schema.CanonicalToolName(entry.namespace, entry.tool.Name())
			name := entry.namespace + "_" + entry.tool.Name()
			if other, exists := seen[name]; exists && other == canonical {
				return nil, nil, llm.ErrBadParameter.Withf("tool %q is named more than once", canonical)
			} else if exists {
				return nil, nil, llm.ErrConflict.Withf("tools %q and %q have the same name %q", other, canonical, name)
			} else if !llm.IsToolName(name) {
				return nil, nil, llm.ErrBadParameter.Withf("tool %q: name %q isn't valid, or is too long", canonical, name)
			}
			seen[name] = canonical
			result = append(result, namedTool{entry: entry, name: name, tracer: self.tracer})
		}
	}
	return result, skipped, nil
}

// ToolNames returns the names, as with Tools, as canonical names, such as
// "weatherapi:current" for "current". A namespace and "*", such as
// "ollama:*", is returned as it is, so it includes tools added to the
// namespace later. Returns the same errors as Tools.
func (self *Toolkit) ToolNames(ctx context.Context, names ...string) (_ []string, err error) {
	ctx, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.ToolNames",
		attribute.StringSlice("arg.names", names),
	)
	defer func() { endSpan(err) }()

	// Check the tools
	if _, err := self.Tools(ctx, names...); err != nil {
		return nil, err
	}

	// Return the canonical names
	result := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if _, ok := strings.CutSuffix(name, schema.ToolSeparator+"*"); ok {
			result = append(result, name)
		} else if entry, err := self.lookup(name); err != nil {
			return nil, err
		} else {
			result = append(result, schema.CanonicalToolName(entry.namespace, entry.tool.Name()))
		}
	}
	return result, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// lookupAll returns the tools with the name, which is a namespace and "*" for
// all the tools in the namespace, in order of canonical name, or otherwise
// the one tool with the name
func (self *Toolkit) lookupAll(name string) ([]entry, error) {
	namespace, ok := strings.CutSuffix(name, schema.ToolSeparator+"*")
	if !ok {
		tool, err := self.lookup(name)
		if err != nil {
			return nil, err
		}
		return []entry{tool}, nil
	}

	self.mu.RLock()
	defer self.mu.RUnlock()
	if !self.namespaces[namespace] {
		return nil, llm.ErrNotFound.Withf("namespace %q", namespace)
	}
	var result []entry
	for _, canonical := range slices.Sorted(maps.Keys(self.tools)) {
		if entry := self.tools[canonical]; entry.namespace == namespace {
			result = append(result, entry)
		}
	}
	return result, nil
}

// lookup returns the tool with the name, which is its canonical name, or its
// name without a namespace, which must be the name of only one tool
func (self *Toolkit) lookup(name string) (entry, error) {
	self.mu.RLock()
	defer self.mu.RUnlock()

	// A canonical name refers to one tool
	if strings.Contains(name, schema.ToolSeparator) {
		if entry, exists := self.tools[name]; exists {
			return entry, nil
		}
		return entry{}, llm.ErrNotFound.Withf("tool %q", name)
	}

	// A name without a namespace refers to the tools with the name in any
	// namespace, in order of canonical name
	var matches []string
	for _, canonical := range slices.Sorted(maps.Keys(self.tools)) {
		if self.tools[canonical].tool.Name() == name {
			matches = append(matches, canonical)
		}
	}
	switch len(matches) {
	case 0:
		return entry{}, llm.ErrNotFound.Withf("tool %q", name)
	case 1:
		return self.tools[matches[0]], nil
	default:
		return entry{}, llm.ErrConflict.Withf("tool %q is ambiguous, use one of: %s", name, strings.Join(matches, ", "))
	}
}

////////////////////////////////////////////////////////////////////////////////
// TYPES

// namedTool is a tool with a new name, which is run as it's called with
// CallTool, so its input is validated, and its result is JSON
type namedTool struct {
	entry  entry
	name   string
	tracer trace.Tracer
}

var _ llm.Tool = namedTool{}

// Name returns the tool's new name
func (t namedTool) Name() string {
	return t.name
}

// CanonicalName returns the tool's canonical name, such as
// "weatherapi:current", rather than the name the model calls it by
func (t namedTool) CanonicalName() string {
	return schema.CanonicalToolName(t.entry.namespace, t.entry.tool.Name())
}

// Description returns the tool's description
func (t namedTool) Description() string {
	return t.entry.tool.Description()
}

// InputSchema returns the JSON schema of the tool's input
func (t namedTool) InputSchema() *jsonschema.Schema {
	return t.entry.tool.InputSchema()
}

// OutputSchema returns the JSON schema of the tool's output, or nil
func (t namedTool) OutputSchema() *jsonschema.Schema {
	return t.entry.tool.OutputSchema()
}

// Run runs the tool with the input, after validating it against the tool's
// input schema, and returns its result as JSON, or nil when it has none, in
// a span, as a model's call of the tool isn't made with CallTool
func (t namedTool) Run(ctx context.Context, input json.RawMessage) (_ any, err error) {
	ctx, endSpan := otel.StartSpan(t.tracer, ctx, "llm.toolkit.Run",
		attribute.String("arg.name", t.CanonicalName()),
	)
	defer func() { endSpan(err) }()

	result, err := t.entry.call(ctx, input)
	if err != nil || result == nil {
		return nil, err
	}
	return result, nil
}
