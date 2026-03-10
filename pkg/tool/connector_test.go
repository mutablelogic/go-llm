package tool_test

import (
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/pkg/mcp/client"
	mock "github.com/mutablelogic/go-llm/pkg/mcp/mock"
	server "github.com/mutablelogic/go-llm/pkg/mcp/server"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// testServer spins up an httptest.Server backed by an MCP server with the
// given mock tools, and returns the test server and a fresh client for it.
// The httptest.Server is closed via t.Cleanup.
func testServer(t *testing.T, tools ...*mock.MockTool) (*httptest.Server, *client.Client) {
	t.Helper()
	srv, err := server.New("test-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	for _, mt := range tools {
		if err := srv.AddTools(mt); err != nil {
			t.Fatal(err)
		}
	}
	ts := httptest.NewServer(srv.Handler())
	c, err := client.New(ts.URL, "test-client", "1.0.0")
	if err != nil {
		ts.Close()
		t.Fatal(err)
	}
	t.Cleanup(ts.Close)
	return ts, c
}

// waitForTools blocks until the toolkit's WithToolsHandler fires with a
// non-nil slice (connector is up) or the deadline is exceeded.
func waitForTools(t *testing.T, ch <-chan []llm.Tool, timeout time.Duration) []llm.Tool {
	t.Helper()
	select {
	case tools := <-ch:
		return tools
	case <-time.After(timeout):
		t.Fatal("timed out waiting for tools from connector")
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestAddConnector_DuplicateURL(t *testing.T) {
	ts, c := testServer(t)

	tk, err := tool.NewToolkit()
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddConnector(ts.URL, c); err != nil {
		t.Fatal("first AddConnector should succeed:", err)
	}
	if err := tk.AddConnector(ts.URL, c); err == nil {
		t.Fatal("expected error for duplicate connector URL")
	}
}

func TestAddConnector_ToolsAppear(t *testing.T) {
	ts, c := testServer(t,
		&mock.MockTool{Name_: "remote_tool", Description_: "A remote tool"},
	)

	toolsCh := make(chan []llm.Tool, 1)
	tk, err := tool.NewToolkit(
		tool.WithToolsHandler(func(url string, tools []llm.Tool) {
			if tools != nil {
				toolsCh <- tools
			}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddConnector(ts.URL, c); err != nil {
		t.Fatal(err)
	}

	tools := waitForTools(t, toolsCh, 5*time.Second)
	if len(tools) != 1 {
		t.Fatalf("expected 1 remote tool, got %d", len(tools))
	}
	if tools[0].Name() != "remote_tool" {
		t.Fatalf("unexpected tool name: %q", tools[0].Name())
	}
}

func TestAddConnector_ListToolsByNamespace(t *testing.T) {
	ts, c := testServer(t,
		&mock.MockTool{Name_: "ns_tool", Description_: "Namespace-filtered tool"},
	)

	toolsCh := make(chan []llm.Tool, 1)
	tk, err := tool.NewToolkit(
		tool.WithToolsHandler(func(url string, tools []llm.Tool) {
			if tools != nil {
				toolsCh <- tools
			}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddConnector(ts.URL, c); err != nil {
		t.Fatal(err)
	}

	// Wait for connection to be established
	waitForTools(t, toolsCh, 5*time.Second)

	// Query by namespace (the URL)
	tools := tk.ListTools(schema.ListToolsRequest{Namespace: ts.URL})
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool under namespace %q, got %d", ts.URL, len(tools))
	}

	// Builtin namespace should not contain connector tools
	builtins := tk.ListTools(schema.ListToolsRequest{Namespace: schema.BuiltinNamespace})
	if len(builtins) != 0 {
		t.Fatalf("expected 0 builtin tools, got %d", len(builtins))
	}
}

func TestAddConnector_LookupRemoteTool(t *testing.T) {
	ts, c := testServer(t,
		&mock.MockTool{Name_: "find_me", Description_: "Remote lookup tool"},
	)

	toolsCh := make(chan []llm.Tool, 1)
	tk, err := tool.NewToolkit(
		tool.WithToolsHandler(func(url string, tools []llm.Tool) {
			if tools != nil {
				toolsCh <- tools
			}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddConnector(ts.URL, c); err != nil {
		t.Fatal(err)
	}

	waitForTools(t, toolsCh, 5*time.Second)

	found := tk.Lookup("find_me")
	if found == nil {
		t.Fatal("expected to find remote tool 'find_me'")
	}
}

func TestRemoveConnector(t *testing.T) {
	ts, c := testServer(t,
		&mock.MockTool{Name_: "gone_tool", Description_: "Will be removed"},
	)

	toolsCh := make(chan []llm.Tool, 2)
	tk, err := tool.NewToolkit(
		tool.WithToolsHandler(func(url string, tools []llm.Tool) {
			toolsCh <- tools
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddConnector(ts.URL, c); err != nil {
		t.Fatal(err)
	}

	// Wait for connection (non-nil tools)
	waitForTools(t, toolsCh, 5*time.Second)

	// Remove the connector — toolkit should fire onTools(nil)
	tk.RemoveConnector(ts.URL)

	select {
	case tools := <-toolsCh:
		if tools != nil {
			t.Fatalf("expected nil tools after disconnect, got %v", tools)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for disconnect notification")
	}

	// After removal, tools should not be visible
	all := tk.ListTools(schema.ListToolsRequest{Namespace: ts.URL})
	if len(all) != 0 {
		t.Fatalf("expected 0 tools after RemoveConnector, got %d", len(all))
	}
}

func TestAddConnector_StateCallback(t *testing.T) {
	ts, c := testServer(t)

	stateCh := make(chan schema.ConnectorState, 1)
	tk, err := tool.NewToolkit(
		tool.WithStateHandler(func(url string, state schema.ConnectorState) {
			if url == ts.URL && state.ConnectedAt != nil && !state.ConnectedAt.IsZero() {
				stateCh <- state
			}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer tk.Close()

	if err := tk.AddConnector(ts.URL, c); err != nil {
		t.Fatal(err)
	}

	select {
	case state := <-stateCh:
		if state.Name == nil || *state.Name != "test-server" {
			t.Fatalf("expected server name 'test-server', got %v", state.Name)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for state callback")
	}
}
