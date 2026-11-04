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
	attribute "go.opentelemetry.io/otel/attribute"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Toolkit is a set of tools, by name, which models can call
type Toolkit struct {
	opt

	// Guards the namespaces and tools, which can be added and removed while
	// the toolkit is in use
	mu sync.RWMutex

	// The namespaces which have been added, by name
	namespaces map[string]Namespace

	// The tools, by canonical name
	tools map[string]entry
}

// Namespace is a namespace of tools: its name, which is an identifier, its
// system prompt, which says how to use its tools, and is added to the system
// prompt of a model which can call any of them, or is empty, and its tools
type Namespace interface {
	Name() string
	SystemPrompt() string
	Tools() []llm.Tool
}

// namespace is a namespace with a name, system prompt and tools
type namespace struct {
	name, prompt string
	tools        []llm.Tool
}

var _ Namespace = namespace{}

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
		namespaces: make(map[string]Namespace),
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

// Add adds the namespace, and its tools, to the toolkit, by their canonical
// name, which is the namespace's name and the tool's name separated by a
// colon, such as "weatherapi:current". The namespace's name must be an
// identifier. Returns an error, and adds none of the tools, if the namespace
// is nil, isn't valid or has already been added, or a tool is nil, its name
// isn't valid, or two tools have the same name.
func (self *Toolkit) Add(ns Namespace) error {
	self.mu.Lock()
	defer self.mu.Unlock()

	// Check the namespace
	if ns == nil {
		return llm.ErrBadParameter.With("missing namespace")
	}
	namespace, tools := ns.Name(), ns.Tools()
	if !types.IsIdentifier(namespace) {
		return llm.ErrBadParameter.Withf("invalid namespace %q", namespace)
	} else if _, exists := self.namespaces[namespace]; exists {
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
	self.namespaces[namespace] = ns
	for name, entry := range entries {
		self.tools[schema.CanonicalToolName(namespace, name)] = entry
	}

	// Return success
	return nil
}

// Credential returns the value of the credential with the key, such as a
// tool's API key, from the toolkit's credential provider. Returns
// ErrNotFound when the credential doesn't exist, or is empty, as when its
// environment variable is set to nothing, so it's as if it isn't set.
func (self *Toolkit) Credential(ctx context.Context, key string) (_ string, err error) {
	ctx, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.Credential",
		attribute.String("arg.key", key),
	)
	defer func() { endSpan(err) }()

	key = strings.TrimSpace(key)
	if key == "" {
		return "", llm.ErrBadParameter.With("missing credential key")
	}
	value, err := self.credentials.Expand(ctx, "${"+key+"}")
	if err != nil {
		return "", err
	} else if strings.TrimSpace(value) == "" {
		return "", llm.ErrNotFound.Withf("credential %q is empty", key)
	}
	return value, nil
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

////////////////////////////////////////////////////////////////////////////////
// NAMESPACE

// NewNamespace returns a namespace with the name, the system prompt, which
// can be empty, and the tools
func NewNamespace(name, prompt string, tools ...llm.Tool) Namespace {
	return namespace{name: name, prompt: prompt, tools: tools}
}

func (n namespace) Name() string {
	return n.name
}

func (n namespace) SystemPrompt() string {
	return n.prompt
}

func (n namespace) Tools() []llm.Tool {
	return n.tools
}
