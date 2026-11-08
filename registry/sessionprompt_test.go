package registry_test

import (
	"context"
	"encoding/json"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// connectedNamespace is a namespace which says which session it was asked
// about, in its system prompt
type connectedNamespace struct {
	toolkit.Namespace
}

func (connectedNamespace) SessionPrompt(_ context.Context, session uuid.UUID) string {
	return "Connected to " + session.String()
}

// pingTool is a tool which does nothing
type pingTool struct{ toolkit.Base }

func (pingTool) Name() string                    { return "ping" }
func (pingTool) Description() string             { return "Does nothing" }
func (pingTool) InputSchema() *jsonschema.Schema { return nil }
func (pingTool) Run(context.Context, json.RawMessage) (any, error) {
	return nil, nil
}

func Test_SessionPrompt(t *testing.T) {
	ctx := context.Background()

	// A namespace's system prompt says what it says of the session a reply
	// is asked in
	tools := test.NewToolkit(t)
	require.NoError(t, tools.Add(connectedNamespace{toolkit.NewNamespace("messaging", "Chats can be connected.", pingTool{})}))
	r, err := registry.New(registry.WithSessionStore(test.NewSessionStore()), registry.WithToolkit(tools))
	require.NoError(t, err)
	p := newScripted(text("Hello"))
	require.NoError(t, r.Add("", p))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Tools: []string{"messaging:*"}}})
	require.NoError(t, err)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
	require.NoError(t, err)
	system, _ := llm.GetOpt[string](p.asked[0].opts, llm.SystemPromptKey)
	assert.Contains(t, system, "Chats can be connected.\n\nConnected to "+session.ID.String())
}
