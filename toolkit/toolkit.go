package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"uuid"

	// Packages
	goauth "github.com/mutablelogic/go-auth"
	client "github.com/mutablelogic/go-client"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	errgroup "golang.org/x/sync/errgroup"
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

	// The tools, by canonical name, and their canonical names, by namespace
	tools map[string]entry
	index map[string]map[string]bool

	// The lifetime of the watches of the namespaces' tools, which ends when
	// the context New is called with is cancelled, or Close is called, and
	// the cancels of each namespace's, by name
	ctx     context.Context
	cancel  context.CancelFunc
	watches map[string]context.CancelFunc
}

// Namespace is a namespace of tools: its name, which is an identifier, a line
// which describes it, such as for a person choosing its tools, which can be
// empty, its system prompt, which says how to use its tools, and is added to
// the system prompt of a model which can call any of them, or is empty, and
// its tools
type Namespace interface {
	Name() string

	// Type returns where the namespace's tools are from: schema.NamespaceBuiltin,
	// for the server's own, schema.NamespaceMCP, for an MCP server's, or
	// schema.NamespaceAgent, for the agents'
	Type() string

	Description() string
	SystemPrompt() string

	// Tools returns the namespace's tools, as they are now, which are the
	// same values, for those which haven't changed, so a toolkit can tell
	// which did
	Tools() []llm.Tool

	// Watch sends the changes to the namespace's tools to the channel, as
	// events whose rows are the tools' names, such as "current", until the
	// context is cancelled, when it returns. It first sends
	// that each of its tools was created, once it's watching, so a change
	// before then isn't missed. An event says which tool to read, with
	// Tools, as it is now, so events can be in any order. It's watched more
	// than once at a time, such as by a toolkit and an MCP server. Watchers
	// does this for a namespace whose tools change, and WatchTools for one
	// whose tools don't.
	Watch(ctx context.Context, ch chan<- eventschema.Change) error
}

// Runner is a namespace which runs, such as to fire what's due, or to watch
// for what happens, until the context is cancelled, when it returns nil. It
// logs to the logger, when there is one. Toolkit.Run runs it.
type Runner interface {
	Run(ctx context.Context, logger *slog.Logger) error
}

// ToolPrompter is a namespace whose system prompt depends on which of its
// tools a model can call, so it says nothing of those it can't
type ToolPrompter interface {
	// SystemPromptFor returns the system prompt, when the model can call
	// the tools, by their names in the namespace, or all of them, when
	// they're nil
	SystemPromptFor(tools []string) string
}

// SessionPrompter is a namespace whose system prompt says more, for a
// session, such as how to use what's connected to it, which is added to its
// system prompt, when a model is asked in the session
type SessionPrompter interface {
	// SessionPrompt returns what's added to the namespace's system prompt,
	// for the session with the identifier, or nothing
	SessionPrompt(ctx context.Context, session uuid.UUID) string
}

// namespace is a namespace with a name, system prompt and tools
type namespace struct {
	name, prompt string
	tools        []llm.Tool
}

var _ Namespace = namespace{}

const (
	// EventSchema is the schema of the toolkit's events, which is the
	// registry's
	EventSchema = "llm"

	// ToolTable is the table of the events of tools, whose rows are their
	// canonical names, such as agent:segment
	ToolTable = "tool"
)

// entry is a tool in a namespace, with its input schema resolved for
// validating its input, or nil if it has no input schema
type entry struct {
	namespace string
	tool      llm.Tool
	input     *jsonschema.Schema
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a toolkit with the options, which watches its namespaces'
// tools until the context is cancelled, or Close is called
func New(ctx context.Context, opts ...Opt) (_ *Toolkit, err error) {
	self := &Toolkit{
		namespaces: make(map[string]Namespace),
		tools:      make(map[string]entry),
		index:      make(map[string]map[string]bool),
		watches:    make(map[string]context.CancelFunc),
	}

	// Apply options
	if err := self.opt.apply(opts); err != nil {
		return nil, err
	}
	self.ctx, self.cancel = context.WithCancel(ctx)

	// Trace creating the toolkit, which needs the tracer from the options
	_, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.New")
	defer func() { endSpan(err) }()

	// With workspaces, it has the tools which list, and read, them
	if self.workspaces != nil {
		if err := self.Add(newWorkspaceNamespace(self)); err != nil {
			return nil, err
		}
	}

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
	if !types.IsIdentifier(namespace) || !schema.IsModelToolPart(namespace) {
		return llm.ErrBadParameter.Withf("invalid namespace %q, which can't have two underscores, or start or end with one", namespace)
	} else if _, exists := self.namespaces[namespace]; exists {
		return llm.ErrConflict.Withf("namespace %q already exists", namespace)
	}

	// Check the tools, and resolve their input schemas, before any are added
	entries := make(map[string]entry, len(tools))
	for _, tool := range tools {
		entry, err := newEntry(namespace, tool)
		if err != nil {
			return err
		} else if _, exists := entries[tool.Name()]; exists {
			return llm.ErrConflict.Withf("duplicate tool %q", tool.Name())
		}
		entries[tool.Name()] = entry
	}

	// A namespace which is a workspace is one of the toolkit's workspaces,
	// too, by its name
	if err := self.addWorkspace(ns); err != nil {
		return err
	}

	// Add the namespace and its tools, which are created, and be told when
	// its tools change
	self.namespaces[namespace] = ns
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		self.set(schema.CanonicalToolName(namespace, name), entries[name])
		self.publish(eventschema.ChangeCreate, schema.CanonicalToolName(namespace, name))
	}
	ctx, cancel := context.WithCancel(self.ctx)
	self.watches[namespace] = cancel
	go self.watch(ctx, ns)

	// Return success
	return nil
}

// Remove removes the namespace, and its tools, from the toolkit, which
// publishes that each of its tools was deleted, when it's the namespace
// with its name, rather than one which replaced it. Returns ErrNotFound
// when it isn't.
func (self *Toolkit) Remove(ns Namespace) error {
	self.mu.Lock()
	defer self.mu.Unlock()
	if ns == nil {
		return llm.ErrBadParameter.With("missing namespace")
	}
	name := ns.Name()
	if !same(self.namespaces[name], ns) {
		return llm.ErrNotFound.Withf("namespace %q", name)
	}
	delete(self.namespaces, name)
	self.removeWorkspace(ns)
	if cancel, exists := self.watches[name]; exists {
		cancel()
		delete(self.watches, name)
	}
	for _, canonical := range slices.Sorted(maps.Keys(self.index[name])) {
		self.unset(canonical)
		self.publish(eventschema.ChangeDelete, canonical)
	}
	return nil
}

// Run runs the namespaces which are Runners, of those which have been added,
// but not any which are added later, until the context is cancelled, and
// they have all returned. When one returns an error, the others are
// cancelled, and it's returned.
func (self *Toolkit) Run(ctx context.Context, logger *slog.Logger) error {
	group, ctx := errgroup.WithContext(ctx)
	self.mu.RLock()
	for _, ns := range self.namespaces {
		if runner, ok := ns.(Runner); ok {
			group.Go(func() error {
				return runner.Run(ctx, logger)
			})
		}
	}
	self.mu.RUnlock()
	return group.Wait()
}

// Close stops watching the namespaces' tools, which are still read as the
// toolkit is asked for them, but whose changes aren't published
func (self *Toolkit) Close() error {
	self.cancel()
	return nil
}

// HasNamespace returns true when the toolkit has a namespace with the name
func (self *Toolkit) HasNamespace(name string) bool {
	self.mu.RLock()
	defer self.mu.RUnlock()
	_, exists := self.namespaces[name]
	return exists
}

// Namespace returns the namespace with the name, or false, when the toolkit
// has none
func (self *Toolkit) Namespace(name string) (Namespace, bool) {
	self.mu.RLock()
	defer self.mu.RUnlock()
	ns, exists := self.namespaces[name]
	return ns, exists
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
	if errors.Is(err, goauth.ErrNotFound) {
		return "", llm.ErrNotFound.Withf("credential %q isn't set", key)
	} else if err != nil {
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

// watch watches the namespace's tools, until the context is cancelled, as
// the namespace is removed, and changes each which changes. When the watch
// fails, the namespace's tools are as they were.
func (self *Toolkit) watch(ctx context.Context, ns Namespace) {
	ch := make(chan eventschema.Change, 16)
	done := make(chan error, 1)
	go func() { done <- ns.Watch(ctx, ch) }()
	for {
		select {
		case event := <-ch:
			self.changed(ns, event.Action, event.Row)
		case <-done:
			return
		}
	}
}

// changed changes the namespace's tools to those its Tools returns now,
// when it's told the tool with the name changed, and publishes that it was
// updated, when it was
func (self *Toolkit) changed(ns Namespace, action eventschema.ChangeAction, name string) {
	self.mu.Lock()
	defer self.mu.Unlock()
	if !same(self.namespaces[ns.Name()], ns) {
		return
	}
	updated := ""
	if action == eventschema.ChangeUpdate {
		updated = name
	}
	self.sync(ns, updated)
}

// current changes each namespace's tools to those its Tools returns now, so
// a tool is called, or listed, as it is, as soon as it's changed, rather
// than once its change is watched. The caller holds the lock.
func (self *Toolkit) current() {
	for _, name := range slices.Sorted(maps.Keys(self.namespaces)) {
		self.sync(self.namespaces[name], "")
	}
}

// currentNamespace changes the tools of the namespace with the name, when
// there is one, as current does. The caller holds the lock.
func (self *Toolkit) currentNamespace(name string) {
	if ns, exists := self.namespaces[name]; exists {
		self.sync(ns, "")
	}
}

// sync changes the namespace's tools to those its Tools returns now, and
// publishes each which is created, updated, or deleted. A tool is updated
// when it's another value than it was, as Tools returns the same value for
// a tool which didn't change. One which can't be compared is changed each
// time, so it's as it is, but is only published as updated when the
// namespace says it was, by its name. One which isn't valid, so can't be
// called, isn't there. The caller holds the lock.
func (self *Toolkit) sync(ns Namespace, updated string) {
	namespace := ns.Name()
	seen := make(map[string]bool)
	for _, tool := range ns.Tools() {
		if tool == nil || seen[schema.CanonicalToolName(namespace, tool.Name())] {
			continue
		}
		canonical := schema.CanonicalToolName(namespace, tool.Name())
		seen[canonical] = true
		existing, existed := self.tools[canonical]
		publish := !existed
		if existed {
			same, ok := sameTool(existing.tool, tool)
			if same {
				continue
			}
			publish = ok || tool.Name() == updated
		}
		entry, err := newEntry(namespace, tool)
		switch {
		case err != nil && existed:
			self.unset(canonical)
			self.publish(eventschema.ChangeDelete, canonical)
		case err != nil:
		case existed:
			self.set(canonical, entry)
			if publish {
				self.publish(eventschema.ChangeUpdate, canonical)
			}
		default:
			self.set(canonical, entry)
			self.publish(eventschema.ChangeCreate, canonical)
		}
	}
	for _, canonical := range slices.Sorted(maps.Keys(self.index[namespace])) {
		if !seen[canonical] {
			self.unset(canonical)
			self.publish(eventschema.ChangeDelete, canonical)
		}
	}
}

// set sets the tool with the canonical name, in its namespace's index. The
// caller holds the lock.
func (self *Toolkit) set(canonical string, e entry) {
	self.tools[canonical] = e
	if self.index[e.namespace] == nil {
		self.index[e.namespace] = make(map[string]bool)
	}
	self.index[e.namespace][canonical] = true
}

// unset removes the tool with the canonical name, and from its namespace's
// index. The caller holds the lock.
func (self *Toolkit) unset(canonical string) {
	e, exists := self.tools[canonical]
	if !exists {
		return
	}
	delete(self.tools, canonical)
	delete(self.index[e.namespace], canonical)
	if len(self.index[e.namespace]) == 0 {
		delete(self.index, e.namespace)
	}
}

// publish publishes that the tool with the canonical name changed, when the
// toolkit has a broadcaster. It never waits, so it's called under the lock,
// so the events are in the order of the changes.
func (self *Toolkit) publish(action eventschema.ChangeAction, canonical string) {
	if self.broadcaster != nil {
		self.broadcaster.Publish(eventschema.Change{Action: action, Schema: EventSchema, Table: ToolTable, Row: canonical})
	}
}

// same returns true when the namespaces are the same one, which they are
// only when they're comparable, such as pointers, rather than panicking,
// as comparing values which can't be does, such as a struct with a slice
func same(a, b Namespace) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	if ta != tb || !ta.Comparable() {
		return false
	}
	return a == b
}

// sameTool returns whether the tools are the same one, which they are
// when they're equal values, and whether that can be told, which it can't
// for values which can't be compared, such as a struct with a slice, or an
// interface whose value is one
func sameTool(a, b llm.Tool) (same, ok bool) {
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	if ta == nil || tb == nil || !ta.Comparable() || !tb.Comparable() {
		return false, false
	} else if ta != tb {
		return false, true
	}
	defer func() {
		if recover() != nil {
			same, ok = false, false
		}
	}()
	return a == b, true
}

// typeOf returns the namespace's type, which is schema.NamespaceBuiltin,
// when it says none
func typeOf(ns Namespace) string {
	if typ := ns.Type(); typ != "" {
		return typ
	}
	return schema.NamespaceBuiltin
}

// namespaceType returns the type of the namespace with the name, or
// schema.NamespaceBuiltin, when there's none
func (self *Toolkit) namespaceType(name string) string {
	self.mu.RLock()
	defer self.mu.RUnlock()
	if ns, exists := self.namespaces[name]; exists {
		return typeOf(ns)
	}
	return schema.NamespaceBuiltin
}

// newEntry returns the tool in the namespace, with its input schema resolved
// for validating its input, or an error when its name, or input schema,
// isn't valid
func newEntry(namespace string, tool llm.Tool) (entry, error) {
	if tool == nil {
		return entry{}, llm.ErrBadParameter.With("missing tool")
	}
	name := tool.Name()
	if !llm.IsToolName(name) || !schema.IsModelToolPart(name) {
		return entry{}, llm.ErrBadParameter.Withf("invalid tool name %q, which can't have two underscores, or start or end with one", name)
	}
	input, err := resolveSchema(tool.InputSchema())
	if err != nil {
		return entry{}, llm.ErrBadParameter.Withf("tool %q: invalid input schema: %v", name, err)
	}
	return entry{namespace: namespace, tool: tool, input: input}, nil
}

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
	return &namespace{name: name, prompt: prompt, tools: tools}
}

func (n namespace) Name() string {
	return n.name
}

// Description returns nothing, as the namespace has no description
func (namespace) Description() string {
	return ""
}

func (n namespace) SystemPrompt() string {
	return n.prompt
}

func (n namespace) Tools() []llm.Tool {
	return n.tools
}

// Type returns that the namespace's tools are built in
func (namespace) Type() string { return schema.NamespaceBuiltin }

// Watch sends the namespace's tools, which don't change
func (n namespace) Watch(ctx context.Context, ch chan<- eventschema.Change) error {
	return WatchTools(ctx, ch, n.tools)
}
