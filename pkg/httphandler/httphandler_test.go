package httphandler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	httphandler "github.com/mutablelogic/go-llm/pkg/httphandler"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK CLIENT

type mockClient struct {
	name   string
	models []schema.Model
}

func (c *mockClient) Name() string { return c.name }

func (c *mockClient) ListModels(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
	// Ensure OwnedBy is set on all models
	result := make([]schema.Model, len(c.models))
	for i, m := range c.models {
		m.OwnedBy = c.name
		result[i] = m
	}
	return result, nil
}

func (c *mockClient) GetModel(_ context.Context, name string, _ ...opt.Opt) (*schema.Model, error) {
	for _, m := range c.models {
		if m.Name == name {
			m.OwnedBy = c.name
			return &m, nil
		}
	}
	return nil, nil
}

///////////////////////////////////////////////////////////////////////////////
// MOCK TOOL

type mockTool struct {
	name        string
	description string
	schema      *jsonschema.Schema
}

func (t *mockTool) Name() string                                          { return t.name }
func (t *mockTool) Description() string                                   { return t.description }
func (t *mockTool) Schema() (*jsonschema.Schema, error)                   { return t.schema, nil }
func (t *mockTool) Run(_ context.Context, _ json.RawMessage) (any, error) { return nil, nil }

///////////////////////////////////////////////////////////////////////////////
// MOCK EMBEDDER CLIENT

// mockEmbedderClient implements both llm.Client and llm.Embedder
type mockEmbedderClient struct {
	mockClient
}

var _ llm.Embedder = (*mockEmbedderClient)(nil)

func (c *mockEmbedderClient) Embedding(_ context.Context, model schema.Model, text string, _ ...opt.Opt) ([]float64, error) {
	return []float64{0.1, 0.2, 0.3}, nil
}

func (c *mockEmbedderClient) BatchEmbedding(_ context.Context, model schema.Model, texts []string, _ ...opt.Opt) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// MOCK GENERATOR CLIENT

// mockGeneratorClient implements both llm.Client and llm.Generator
type mockGeneratorClient struct {
	mockClient
}

var _ llm.Generator = (*mockGeneratorClient)(nil)

func (c *mockGeneratorClient) WithoutSession(_ context.Context, _ schema.Model, msg *schema.Message, _ ...opt.Opt) (*schema.Message, *schema.Usage, error) {
	return &schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{
			{Text: types.Ptr("ask response: " + msg.Text())},
		},
		Result: schema.ResultOK,
	}, &schema.Usage{}, nil
}

func (c *mockGeneratorClient) WithSession(_ context.Context, _ schema.Model, _ *schema.Conversation, msg *schema.Message, _ ...opt.Opt) (*schema.Message, *schema.Usage, error) {
	return &schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{
			{Text: types.Ptr("chat response: " + msg.Text())},
		},
		Result: schema.ResultOK,
	}, &schema.Usage{}, nil
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func newTestManager(t *testing.T, clients []mockClient, tools ...tool.Tool) *agent.Manager {
	t.Helper()
	var opts []agent.Opt
	for i := range clients {
		opts = append(opts, agent.WithClient(&clients[i]))
	}
	if len(tools) > 0 {
		tk, err := tool.NewToolkit(tools...)
		if err != nil {
			t.Fatal(err)
		}
		opts = append(opts, agent.WithToolkit(tk))
	}
	m, err := agent.NewManager(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func newTestManagerWithGenerator(t *testing.T, clients []*mockGeneratorClient, tools ...tool.Tool) *agent.Manager {
	t.Helper()
	var opts []agent.Opt
	for _, c := range clients {
		opts = append(opts, agent.WithClient(c))
	}
	if len(tools) > 0 {
		tk, err := tool.NewToolkit(tools...)
		if err != nil {
			t.Fatal(err)
		}
		opts = append(opts, agent.WithToolkit(tk))
	}
	m, err := agent.NewManager(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func serveMux(manager *agent.Manager) *http.ServeMux {
	mux := http.NewServeMux()
	path, handler, _ := httphandler.ModelListHandler(manager)
	mux.HandleFunc(path, handler)
	path, handler, _ = httphandler.ModelGetHandler(manager)
	mux.HandleFunc(path, handler)
	path, handler, _ = httphandler.ToolListHandler(manager)
	mux.HandleFunc(path, handler)
	path, handler, _ = httphandler.ToolGetHandler(manager)
	mux.HandleFunc(path, handler)
	path, handler, _ = httphandler.EmbeddingHandler(manager)
	mux.HandleFunc(path, handler)
	path, handler, _ = httphandler.SessionHandler(manager)
	mux.HandleFunc(path, handler)
	path, handler, _ = httphandler.SessionGetHandler(manager)
	mux.HandleFunc(path, handler)
	path, handler, _ = httphandler.AskHandler(manager)
	mux.HandleFunc(path, handler)
	path, handler, _ = httphandler.ChatHandler(manager)
	mux.HandleFunc(path, handler)
	return mux
}
