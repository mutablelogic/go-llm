package manager

import (
	"context"
	"encoding/json"
	"fmt"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type createMemoryTool struct {
	tool.DefaultTool
	manager *Manager
}

type searchMemoryTool struct {
	tool.DefaultTool
	manager *Manager
}

type createMemoryRequest struct {
	Key   string `json:"key" jsonschema:"Memory key to create."`
	Value string `json:"value" jsonschema:"Text value stored under the key."`
}

type searchMemoryRequest struct {
	Q string `json:"q" jsonschema:"Text query used to search memory keys and values."`
}

var _ llm.Tool = (*createMemoryTool)(nil)
var _ llm.Tool = (*searchMemoryTool)(nil)

// Run establishes and drives the connection until ctx is cancelled
// or the remote server closes it.
func (m *Manager) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// ListPrompts returns all prompts advertised by the connected remote server.
func (m *Manager) ListPrompts(ctx context.Context) ([]llm.Prompt, error) {
	return nil, nil
}

// ListResources returns all resources advertised by the connected remote server.
func (m *Manager) ListResources(ctx context.Context) ([]llm.Resource, error) {
	return nil, nil
}

// ListTools returns all tools advertised by the connected remote server.
func (m *Manager) ListTools(ctx context.Context) ([]llm.Tool, error) {
	return []llm.Tool{
		&createMemoryTool{manager: m},
		&searchMemoryTool{manager: m},
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func memorySessionFromContext(ctx context.Context) (uuid.UUID, error) {
	id := toolkit.SessionFromContext(ctx).ID()
	if id == "" {
		return uuid.Nil, fmt.Errorf("memory tool requires a session id in context")
	}
	session, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("memory tool session id %q is invalid: %w", id, err)
	}
	return session, nil
}

///////////////////////////////////////////////////////////////////////////////
// llm.Tool - memory_write

func (*createMemoryTool) Name() string {
	return "memory_write"
}

func (*createMemoryTool) Description() string {
	return "Create a memory entry for the current session using a key and text value."
}

func (*createMemoryTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[createMemoryRequest]()
}

func (*createMemoryTool) OutputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[schema.Memory]()
}

func (t *createMemoryTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req createMemoryRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("memory_write: decode input: %w", err)
		}
	}
	session, err := memorySessionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return t.manager.CreateMemory(ctx, schema.MemoryInsert{
		Session: session,
		Key:     req.Key,
		MemoryMeta: schema.MemoryMeta{
			Value: &req.Value,
		},
	})
}

///////////////////////////////////////////////////////////////////////////////
// llm.Tool - memory_search

func (*searchMemoryTool) Name() string {
	return "memory_search"
}

func (*searchMemoryTool) Description() string {
	return "Search memory entries for the current session using a text query."
}

func (*searchMemoryTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[searchMemoryRequest]()
}

func (*searchMemoryTool) OutputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[schema.MemoryList]()
}

func (searchMemoryTool) Meta() llm.ToolMeta {
	return llm.ToolMeta{ReadOnlyHint: true, IdempotentHint: true}
}

func (t *searchMemoryTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req searchMemoryRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("memory_search: decode input: %w", err)
		}
	}
	session, err := memorySessionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return t.manager.ListMemory(ctx, schema.MemoryListRequest{
		Session: &session,
		Q:       req.Q,
	})
}
