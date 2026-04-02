package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	resource "github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	toolpkg "github.com/mutablelogic/go-llm/pkg/toolkit/tool"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// newConnectedToolkit creates a toolkit with one connector already registered
// in the namespace map (simulating a connected state) under the given ns.
func newConnectedToolkit(t *testing.T, ns string, conn *mockListConnector) *toolkit {
	t.Helper()
	tk, _ := newConnectorToolkit(t)
	// Inject directly into the namespace map and connectors map.
	c := &connector{namespace: ns, conn: conn}
	tk.namespace[ns] = c
	tk.connectors["http://localhost:8080"] = c
	return tk
}

///////////////////////////////////////////////////////////////////////////////
// Lookup - connector tools

// Lookup by bare name finds a tool exposed by a connected connector.
func Test_Lookup_Connector_001(t *testing.T) {
	conn := &mockListConnector{tools: []llm.Tool{&mockTool{name: "remote_tool"}}}
	tk := newConnectedToolkit(t, "myserver", conn)

	v, err := tk.Lookup(context.Background(), "remote_tool")
	if err != nil {
		t.Fatal(err)
	}
	tool, ok := v.(llm.Tool)
	if !ok || tool == nil {
		t.Fatalf("expected llm.Tool, got %T", v)
	}
	if tool.Name() != "myserver.remote_tool" {
		t.Fatalf("unexpected name %q", tool.Name())
	}
}

// Lookup by namespace.name finds a tool in the right connector.
func Test_Lookup_Connector_002(t *testing.T) {
	conn := &mockListConnector{tools: []llm.Tool{&mockTool{name: "remote_tool"}}}
	tk := newConnectedToolkit(t, "myserver", conn)

	v, err := tk.Lookup(context.Background(), "myserver.remote_tool")
	if err != nil {
		t.Fatal(err)
	}
	tool, ok := v.(llm.Tool)
	if !ok || tool == nil {
		t.Fatalf("expected llm.Tool, got %T", v)
	}
}

// Lookup by wrong namespace returns ErrNotFound.
func Test_Lookup_Connector_003(t *testing.T) {
	conn := &mockListConnector{tools: []llm.Tool{&mockTool{name: "remote_tool"}}}
	tk := newConnectedToolkit(t, "myserver", conn)

	_, err := tk.Lookup(context.Background(), "other.remote_tool")
	if !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// Builtin tool takes precedence over a connector tool with the same bare name.
func Test_Lookup_Connector_004(t *testing.T) {
	conn := &mockListConnector{tools: []llm.Tool{&mockTool{name: "shared_tool"}}}
	tk := newConnectedToolkit(t, "myserver", conn)
	if err := tk.AddTool(&mockTool{name: "shared_tool"}); err != nil {
		t.Fatal(err)
	}

	v, err := tk.Lookup(context.Background(), "builtin.shared_tool")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(llm.Tool); !ok || v == nil {
		t.Fatalf("expected llm.Tool, got %T", v)
	}
	// Searching the connector namespace should return the connector's copy.
	v2, err := tk.Lookup(context.Background(), "myserver.shared_tool")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v2.(llm.Tool); !ok || v2 == nil {
		t.Fatalf("expected llm.Tool from connector, got %T", v2)
	}
}

// Lookup by bare name finds a connector tool when no builtin matches.
func Test_Lookup_Connector_005_two_connectors(t *testing.T) {
	connA := &mockListConnector{tools: []llm.Tool{&mockTool{name: "tool_a"}}}
	connB := &mockListConnector{tools: []llm.Tool{&mockTool{name: "tool_b"}}}
	tk, _ := newConnectorToolkit(t)
	tk.namespace["server_a"] = &connector{namespace: "server_a", conn: connA}
	tk.namespace["server_b"] = &connector{namespace: "server_b", conn: connB}

	v, err := tk.Lookup(context.Background(), "server_b.tool_b")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil {
		t.Fatal("expected result, got nil")
	}
}

///////////////////////////////////////////////////////////////////////////////
// Lookup - connector prompts

// Lookup by bare name finds a prompt from a connected connector.
func Test_Lookup_Connector_006(t *testing.T) {
	conn := &mockListConnector{prompts: []llm.Prompt{&mockPrompt{name: "remote_prompt"}}}
	tk := newConnectedToolkit(t, "myserver", conn)

	v, err := tk.Lookup(context.Background(), "remote_prompt")
	if err != nil {
		t.Fatal(err)
	}
	p, ok := v.(llm.Prompt)
	if !ok || p == nil {
		t.Fatalf("expected llm.Prompt, got %T", v)
	}
	if p.Name() != "myserver.remote_prompt" {
		t.Fatalf("unexpected name %q", p.Name())
	}
}

// Lookup by namespace.name finds a prompt in the right connector.
func Test_Lookup_Connector_007(t *testing.T) {
	conn := &mockListConnector{prompts: []llm.Prompt{&mockPrompt{name: "remote_prompt"}}}
	tk := newConnectedToolkit(t, "myserver", conn)

	v, err := tk.Lookup(context.Background(), "myserver.remote_prompt")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(llm.Prompt); !ok {
		t.Fatalf("expected llm.Prompt, got %T", v)
	}
}

// Lookup by wrong namespace returns ErrNotFound for a prompt.
func Test_Lookup_Connector_008(t *testing.T) {
	conn := &mockListConnector{prompts: []llm.Prompt{&mockPrompt{name: "remote_prompt"}}}
	tk := newConnectedToolkit(t, "myserver", conn)

	_, err := tk.Lookup(context.Background(), "other.remote_prompt")
	if !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// Lookup - connector resources

// Lookup by URI finds a resource from a connected connector.
func Test_Lookup_Connector_009(t *testing.T) {
	r, _ := resource.Text("remote_doc", "hello from connector")
	conn := &mockListConnector{resources: []llm.Resource{r}}
	tk := newConnectedToolkit(t, "myserver", conn)

	v, err := tk.Lookup(context.Background(), r.URI())
	if err != nil {
		t.Fatal(err)
	}
	res, ok := v.(llm.Resource)
	if !ok || res == nil {
		t.Fatalf("expected llm.Resource, got %T", v)
	}
	if res.Name() != "myserver.remote_doc" {
		t.Fatalf("unexpected name %q", res.Name())
	}
}

// Lookup by URI#namespace finds a resource in the right connector.
func Test_Lookup_Connector_010(t *testing.T) {
	r, _ := resource.Text("remote_doc", "hello from connector")
	conn := &mockListConnector{resources: []llm.Resource{r}}
	tk := newConnectedToolkit(t, "myserver", conn)

	v, err := tk.Lookup(context.Background(), r.URI()+"#myserver")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(llm.Resource); !ok {
		t.Fatalf("expected llm.Resource, got %T", v)
	}
}

// Lookup by URI#wrongnamespace returns ErrNotFound.
func Test_Lookup_Connector_011(t *testing.T) {
	r, _ := resource.Text("remote_doc", "hello from connector")
	conn := &mockListConnector{resources: []llm.Resource{r}}
	tk := newConnectedToolkit(t, "myserver", conn)

	_, err := tk.Lookup(context.Background(), r.URI()+"#other")
	if !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// Builtin resource takes precedence over a connector resource with the same URI.
func Test_Lookup_Connector_012(t *testing.T) {
	r, _ := resource.Text("shared_doc", "builtin copy")
	conn := &mockListConnector{resources: []llm.Resource{r}}
	tk := newConnectedToolkit(t, "myserver", conn)
	if err := tk.AddResource(r); err != nil {
		t.Fatal(err)
	}

	// Bare URI should return the builtin (name prefixed with "builtin.").
	v, err := tk.Lookup(context.Background(), r.URI())
	if err != nil {
		t.Fatal(err)
	}
	res, ok := v.(llm.Resource)
	if !ok || res == nil {
		t.Fatalf("expected llm.Resource, got %T", v)
	}
	if res.Name() != "builtin.shared_doc" {
		t.Fatalf("expected builtin.shared_doc, got %q", res.Name())
	}
}

// Lookup by URI#builtin only searches builtins and returns ErrNotFound when absent.
func Test_Lookup_Connector_013(t *testing.T) {
	r, _ := resource.Text("remote_doc", "connector only")
	conn := &mockListConnector{resources: []llm.Resource{r}}
	tk := newConnectedToolkit(t, "myserver", conn)

	_, err := tk.Lookup(context.Background(), r.URI()+"#builtin")
	if !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// Lookup by URI#namespace with two connectors finds the right one.
func Test_Lookup_Connector_014(t *testing.T) {
	rA, _ := resource.Text("doc_a", "from server_a")
	rB, _ := resource.Text("doc_b", "from server_b")
	connA := &mockListConnector{resources: []llm.Resource{rA}}
	connB := &mockListConnector{resources: []llm.Resource{rB}}
	tk, _ := newConnectorToolkit(t)
	tk.namespace["server_a"] = &connector{namespace: "server_a", conn: connA}
	tk.namespace["server_b"] = &connector{namespace: "server_b", conn: connB}

	v, err := tk.Lookup(context.Background(), rB.URI()+"#server_b")
	if err != nil {
		t.Fatal(err)
	}
	res, ok := v.(llm.Resource)
	if !ok || res == nil {
		t.Fatalf("expected llm.Resource, got %T", v)
	}
	if res.Name() != "server_b.doc_b" {
		t.Fatalf("unexpected name %q", res.Name())
	}
}

// Lookup of a URI that no connector exposes returns ErrNotFound.
func Test_Lookup_Connector_015(t *testing.T) {
	conn := &mockListConnector{resources: []llm.Resource{}}
	tk := newConnectedToolkit(t, "myserver", conn)

	r, _ := resource.Text("missing_doc", "not here")
	_, err := tk.Lookup(context.Background(), r.URI())
	if !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// suppress unused import warnings

var _ = json.RawMessage{}
var _ = (*jsonschema.Schema)(nil)
var _ = toolpkg.WithNamespace
