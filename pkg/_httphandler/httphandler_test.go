package httphandler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	httphandler "github.com/mutablelogic/go-llm/pkg/httphandler"
	manager "github.com/mutablelogic/go-llm/pkg/manager"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
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
func (t *mockTool) InputSchema() (*jsonschema.Schema, error)              { return t.schema, nil }
func (t *mockTool) OutputSchema() (*jsonschema.Schema, error)             { return nil, nil }
func (t *mockTool) Meta() llm.ToolMeta                                    { return llm.ToolMeta{} }
func (t *mockTool) Run(_ context.Context, _ json.RawMessage) (any, error) { return nil, nil }

///////////////////////////////////////////////////////////////////////////////
// MOCK DOWNLOADER CLIENT

// mockDownloaderClient implements llm.Client and llm.Downloader.
type mockDownloaderClient struct {
	mockClient
}

var _ llm.Downloader = (*mockDownloaderClient)(nil)

func (c *mockDownloaderClient) DownloadModel(_ context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	// Call the progress fn if one was provided
	if options, err := opt.Apply(opts...); err == nil {
		if progressFn := options.GetProgress(); progressFn != nil {
			progressFn("pulling", 50.0)
			progressFn("done", 100.0)
		}
	}
	return &schema.Model{Name: name, OwnedBy: c.name}, nil
}

func (c *mockDownloaderClient) DeleteModel(_ context.Context, model schema.Model) error {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// MOCK EMBEDDER CLIENT

// mockEmbedderClient implements both llm.Client and llm.Embedder
type mockEmbedderClient struct {
	mockClient
}

var _ llm.Embedder = (*mockEmbedderClient)(nil)

func (c *mockEmbedderClient) Embedding(_ context.Context, model schema.Model, text string, _ ...opt.Opt) ([]float64, *schema.UsageMeta, error) {
	return []float64{0.1, 0.2, 0.3}, nil, nil
}

func (c *mockEmbedderClient) BatchEmbedding(_ context.Context, model schema.Model, texts []string, _ ...opt.Opt) ([][]float64, *schema.UsageMeta, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil, nil
}

///////////////////////////////////////////////////////////////////////////////
// MOCK GENERATOR CLIENT

// mockGeneratorClient implements both llm.Client and llm.Generator
type mockGeneratorClient struct {
	mockClient
}

var _ llm.Generator = (*mockGeneratorClient)(nil)

func (c *mockGeneratorClient) WithoutSession(_ context.Context, _ schema.Model, msg *schema.Message, _ ...opt.Opt) (*schema.Message, *schema.UsageMeta, error) {
	return &schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{
			{Text: types.Ptr("ask response: " + msg.Text())},
		},
		Result: schema.ResultOK,
	}, &schema.UsageMeta{}, nil
}

func (c *mockGeneratorClient) WithSession(_ context.Context, _ schema.Model, _ *schema.Conversation, msg *schema.Message, _ ...opt.Opt) (*schema.Message, *schema.UsageMeta, error) {
	return &schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{
			{Text: types.Ptr("chat response: " + msg.Text())},
		},
		Result: schema.ResultOK,
	}, &schema.UsageMeta{}, nil
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func newTestManager(t *testing.T, clients []mockClient, tools ...llm.Tool) *manager.Manager {
	t.Helper()
	var opts []manager.Opt
	for i := range clients {
		opts = append(opts, manager.WithClient(&clients[i]))
	}
	if len(tools) > 0 {
		opts = append(opts, manager.WithTools(tools...))
	}
	m, err := manager.NewManager("test", "0.0.0", opts...)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func newTestManagerWithDownloader(t *testing.T, clients []*mockDownloaderClient) *manager.Manager {
	t.Helper()
	var opts []manager.Opt
	for _, c := range clients {
		opts = append(opts, manager.WithClient(c))
	}
	m, err := manager.NewManager("test", "0.0.0", opts...)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func newTestManagerWithGenerator(t *testing.T, clients []*mockGeneratorClient, tools ...llm.Tool) *manager.Manager {
	t.Helper()
	var opts []manager.Opt
	for _, c := range clients {
		opts = append(opts, manager.WithClient(c))
	}
	if len(tools) > 0 {
		opts = append(opts, manager.WithTools(tools...))
	}
	m, err := manager.NewManager("test", "0.0.0", opts...)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func serveMux(manager *manager.Manager) *http.ServeMux {
	mux := http.NewServeMux()
	register := func(path string, handler http.HandlerFunc, _ any) {
		mux.HandleFunc("/"+path, handler)
	}
	register(httphandler.ModelListHandler(manager))
	register(httphandler.ModelGetHandler(manager))
	register(httphandler.ToolListHandler(manager))
	register(httphandler.ToolGetHandler(manager))
	register(httphandler.EmbeddingHandler(manager))
	register(httphandler.SessionHandler(manager))
	register(httphandler.SessionGetHandler(manager))
	register(httphandler.AgentHandler(manager))
	register(httphandler.AgentGetHandler(manager))
	register(httphandler.AskHandler(manager))
	register(httphandler.ChatHandler(manager))
	register(httphandler.ConnectorListHandler(manager))
	register(httphandler.ConnectorHandler(manager))
	return mux
}
