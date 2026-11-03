package toolkit

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	// Packages
	client "github.com/mutablelogic/go-client"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Toolkit is a set of tools, by name, which models can call
type Toolkit struct {
	opt

	// Guards the namespaces and tools, which can be added and removed while
	// the toolkit is in use
	mu sync.RWMutex

	// The namespaces which have been added
	namespaces map[string]bool

	// The tools, by canonical name
	tools map[string]entry
}

// entry is a tool in a namespace, with its input schema resolved for
// validating its input, or nil if it has no input schema
type entry struct {
	namespace string
	tool      llm.Tool
	input     *jsonschema.Schema
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a toolkit with the options
func New(ctx context.Context, opts ...Opt) (_ *Toolkit, err error) {
	self := &Toolkit{
		namespaces: make(map[string]bool),
		tools:      make(map[string]entry),
	}

	// Apply options
	if err := self.opt.apply(opts); err != nil {
		return nil, err
	}

	// Trace creating the toolkit, which needs the tracer from the options
	_, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.New")
	defer func() { endSpan(err) }()

	// Return success
	return self, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Add adds the tools in the namespace to the toolkit, by their canonical
// name, which is the namespace and the tool's name separated by a colon,
// such as "weatherapi:current". The namespace must be an
// identifier. Returns an error, and adds none of the tools, if the namespace
// isn't valid or has already been added, or a tool is nil, its name isn't
// valid, or two tools have the same name.
func (self *Toolkit) Add(namespace string, tools ...llm.Tool) error {
	self.mu.Lock()
	defer self.mu.Unlock()

	// Check the namespace
	if !types.IsIdentifier(namespace) {
		return llm.ErrBadParameter.Withf("invalid namespace %q", namespace)
	} else if self.namespaces[namespace] {
		return llm.ErrConflict.Withf("namespace %q already exists", namespace)
	}

	// Check the tools, and resolve their input schemas, before any are added
	entries := make(map[string]entry, len(tools))
	for _, tool := range tools {
		if tool == nil {
			return llm.ErrBadParameter.With("missing tool")
		}
		name := tool.Name()
		if !llm.IsToolName(name) {
			return llm.ErrBadParameter.Withf("invalid tool name %q", name)
		} else if _, exists := entries[name]; exists {
			return llm.ErrConflict.Withf("duplicate tool %q", name)
		}
		input, err := resolveSchema(tool.InputSchema())
		if err != nil {
			return llm.ErrBadParameter.Withf("tool %q: invalid input schema: %v", name, err)
		}
		entries[name] = entry{namespace: namespace, tool: tool, input: input}
	}

	// Add the namespace and its tools
	self.namespaces[namespace] = true
	for name, entry := range entries {
		self.tools[schema.CanonicalToolName(namespace, name)] = entry
	}

	// Return success
	return nil
}

// Credential returns the value of the credential with the key, such as a
// tool's API key, from the toolkit's credential provider
func (self *Toolkit) Credential(ctx context.Context, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", llm.ErrBadParameter.With("missing credential key")
	}
	return self.credentials.Expand(ctx, "${"+key+"}")
}

// ClientOpts returns the client options for the HTTP clients of tools which
// call APIs
func (self *Toolkit) ClientOpts() []client.ClientOpt {
	return self.clientopts
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// resolveSchema returns the schema resolved for validation, or nil if it's
// nil. A schema is resolved from its JSON, so a schema which wasn't created
// with jsonschema.For or jsonschema.FromJSON can be used for validation.
func resolveSchema(v *jsonschema.Schema) (*jsonschema.Schema, error) {
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return jsonschema.FromJSON(data)
}
