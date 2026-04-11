package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type createMemoryTool struct {
	tool.Base
	manager *Manager
}

type searchMemoryTool struct {
	tool.Base
	manager *Manager
}

type createMemoryRequest struct {
	Key   string `json:"key" jsonschema:"Create a persistent fact. Use 'name' for the user's name, 'location', 'timezone', 'time', 'date' and so forth."`
	Value string `json:"value" jsonschema:"Text value stored under the key."`
}

type searchMemoryRequest struct {
	Q string `json:"q" jsonschema:"Web-style text query used to search user information keys and values. Leave empty or use * to list all memories for the current session."`
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
	session := toolkit.SessionFromContext(ctx)
	session.Logger().InfoContext(ctx, "memorySessionFromContext", "session", session, "session_id", session.ID(), "meta", session.Meta())
	if id := session.ID(); id == "" {
		return uuid.Nil, fmt.Errorf("memory tool requires a session id in context")
	} else if id, err := uuid.Parse(id); err != nil {
		return uuid.Nil, fmt.Errorf("memory tool session id %q is invalid: %w", id, err)
	} else {
		return id, nil
	}
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
	return "Search memory entries for the current session using PostgreSQL web-style search syntax. Leave q empty or use * to list all memories for the session. Current UTC date, time, datetime, and timezone are returned as dynamic memory entries."
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
	result, err := t.manager.ListMemory(ctx, schema.MemoryListRequest{
		Session: &session,
		Q:       req.Q,
	})
	if err != nil {
		return nil, err
	}
	return mergeDynamicMemory(result, session, req.Q, time.Now()), nil
}

func mergeDynamicMemory(list *schema.MemoryList, session uuid.UUID, q string, now time.Time) *schema.MemoryList {
	if list == nil {
		list = &schema.MemoryList{}
	}
	dynamic := filterDynamicMemory(dynamicMemoryEntries(session, now), q)
	if len(dynamic) == 0 {
		return list
	}

	dynamicKeys := make(map[string]struct{}, len(dynamic))
	for _, memory := range dynamic {
		if memory == nil {
			continue
		}
		dynamicKeys[strings.TrimSpace(memory.Key)] = struct{}{}
	}

	body := make([]*schema.Memory, 0, len(dynamic)+len(list.Body))
	body = append(body, dynamic...)
	for _, memory := range list.Body {
		if memory == nil {
			continue
		}
		if _, exists := dynamicKeys[strings.TrimSpace(memory.Key)]; exists {
			continue
		}
		body = append(body, memory)
	}
	list.Body = body
	list.Count = uint(len(body))
	return list
}

func dynamicMemoryEntries(session uuid.UUID, now time.Time) []*schema.Memory {
	now = now.UTC()
	date := now.Format("2006-01-02")
	timeValue := now.Format("15:04:05 MST")
	datetime := now.Format(time.RFC3339)
	timezone := now.Location().String()

	return []*schema.Memory{
		{MemoryInsert: schema.MemoryInsert{Session: session, Key: "date", MemoryMeta: schema.MemoryMeta{Value: &date}}, CreatedAt: now},
		{MemoryInsert: schema.MemoryInsert{Session: session, Key: "time", MemoryMeta: schema.MemoryMeta{Value: &timeValue}}, CreatedAt: now},
		{MemoryInsert: schema.MemoryInsert{Session: session, Key: "datetime", MemoryMeta: schema.MemoryMeta{Value: &datetime}}, CreatedAt: now},
		{MemoryInsert: schema.MemoryInsert{Session: session, Key: "timezone", MemoryMeta: schema.MemoryMeta{Value: &timezone}}, CreatedAt: now},
	}
}

func filterDynamicMemory(memories []*schema.Memory, q string) []*schema.Memory {
	q = strings.TrimSpace(q)
	if q == "" || q == "*" {
		return memories
	}
	tokens := dynamicMemoryTokens(q)
	if len(tokens) == 0 {
		return memories
	}

	result := make([]*schema.Memory, 0, len(memories))
	for _, memory := range memories {
		if memory == nil {
			continue
		}
		haystack := dynamicMemoryTokens(strings.TrimSpace(memory.Key) + " " + strings.TrimSpace(valueOrEmpty(memory.Value)))
		for _, token := range tokens {
			if containsToken(haystack, token) {
				result = append(result, memory)
				break
			}
		}
	}
	return result
}

func dynamicMemoryTokens(q string) []string {
	fields := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		default:
			return true
		}
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		switch field {
		case "and", "or", "not":
			continue
		}
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func containsToken(tokens []string, target string) bool {
	for _, token := range tokens {
		if token == target {
			return true
		}
	}
	return false
}
