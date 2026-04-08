package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	mock "github.com/mutablelogic/go-llm/mcp/mock"
	serverpkg "github.com/mutablelogic/go-llm/mcp/server"
	promptpkg "github.com/mutablelogic/go-llm/toolkit/prompt"
	resourcepkg "github.com/mutablelogic/go-llm/toolkit/resource"
	metric "go.opentelemetry.io/otel/metric"
	trace "go.opentelemetry.io/otel/trace"
)

type testCmd struct {
	ctx context.Context
	log *slog.Logger
	url string
}

type namedReader struct {
	*bytes.Reader
	name string
}

func (r *namedReader) Name() string { return r.name }

func (c *testCmd) Name() string                                        { return "mcp-test" }
func (c *testCmd) Description() string                                 { return "test" }
func (c *testCmd) Version() string                                     { return "1.0.0" }
func (c *testCmd) Context() context.Context                            { return c.ctx }
func (c *testCmd) Logger() *slog.Logger                                { return c.log }
func (c *testCmd) Tracer() trace.Tracer                                { return nil }
func (c *testCmd) Meter() metric.Meter                                 { return nil }
func (c *testCmd) ClientEndpoint() (string, []client.ClientOpt, error) { return c.url, nil, nil }
func (c *testCmd) Get(string) any                                      { return nil }
func (c *testCmd) GetString(string) string                             { return "" }
func (c *testCmd) Set(string, any) error                               { return nil }
func (c *testCmd) Keys() []string                                      { return nil }
func (c *testCmd) IsTerm() int                                         { return 0 }
func (c *testCmd) IsDebug() bool                                       { return false }
func (c *testCmd) HTTPAddr() string                                    { return "" }
func (c *testCmd) HTTPPrefix() string                                  { return "" }
func (c *testCmd) HTTPTimeout() time.Duration                          { return 0 }

func mustReadPrompt(t *testing.T, name, body string) llm.Prompt {
	t.Helper()
	p, err := promptpkg.Read(&namedReader{Reader: bytes.NewReader([]byte(body)), name: name})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	_ = w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func newCommandTestContext(t *testing.T) (*testCmd, *serverpkg.Server, *httptest.Server) {
	t.Helper()
	srv, err := serverpkg.New("test-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return &testCmd{ctx: context.Background(), log: slog.New(slog.NewTextHandler(io.Discard, nil)), url: ts.URL}, srv, ts
}

func TestCallToolCommandRequestInvalidJSON(t *testing.T) {
	cmd := CallToolCommand{Name: "tool", Input: `{"query":`, URLFlag: URLFlag{URL: "http://example.com"}}
	if _, err := cmd.requestWithInput(nil, false); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestGetPromptCommandArgumentsFromStdin(t *testing.T) {
	cmd := GetPromptCommand{Name: "prompt", URLFlag: URLFlag{URL: "http://example.com"}}
	got, err := cmd.argumentsWithInput(bytes.NewBufferString(`{"name":"World"}`), true)
	if err != nil {
		t.Fatal(err)
	}
	if got["name"] != "World" {
		t.Fatalf("expected name=World, got %+v", got)
	}
}

func TestGetPromptCommandRun(t *testing.T) {
	ctx, srv, _ := newCommandTestContext(t)
	srv.AddPrompts(mustReadPrompt(t, "greet.md", `---
name: greet
input:
  type: object
  properties:
    name:
      type: string
  required:
    - name
---
Hello, {{ .name }}!`))

	output := captureStdout(t, func() {
		cmd := GetPromptCommand{URLFlag: URLFlag{URL: ctx.url}, Name: "greet", Input: `{"name":"World"}`}
		if err := cmd.Run(ctx); err != nil {
			t.Fatal(err)
		}
	})
	if output != "Hello, World!\n" {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestGetResourceCommandRun(t *testing.T) {
	ctx, srv, _ := newCommandTestContext(t)
	if err := srv.AddResources(resourcepkg.WithURI("memory://hello", resourcepkg.Must("hello", "hello resource"))); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		cmd := GetResourceCommand{URLFlag: URLFlag{URL: ctx.url}, URI: "memory://hello"}
		if err := cmd.Run(ctx); err != nil {
			t.Fatal(err)
		}
	})
	if output != "hello resource\n" {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestCallToolCommandRun(t *testing.T) {
	ctx, srv, _ := newCommandTestContext(t)
	if err := srv.AddTools(&mock.MockTool{Name_: "echo", Result_: map[string]any{"ok": true}}); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		cmd := CallToolCommand{URLFlag: URLFlag{URL: ctx.url}, Name: "echo"}
		if err := cmd.Run(ctx); err != nil {
			t.Fatal(err)
		}
	})
	if output == "" {
		t.Fatal("expected output")
	}
}

func TestWriteJSONTableArray(t *testing.T) {
	data := json.RawMessage(`[
	  {"entity_id":"light.kitchen","name":"Kitchen","state":"on","unit":""},
	  {"entity_id":"sensor.temp","name":"Temperature","state":"21.5","unit":"C"}
	]`)

	output := captureStdout(t, func() {
		if err := writeJSON(os.Stdout, data); err != nil {
			t.Fatal(err)
		}
	})

	for _, want := range []string{"ENTITY_ID", "NAME", "STATE", "light.kitchen", "sensor.temp", "2 item(s)"} {
		if !strings.Contains(strings.ToUpper(output), strings.ToUpper(want)) {
			t.Fatalf("expected output to contain %q, got %q", want, output)
		}
	}
	if strings.Contains(output, "[") {
		t.Fatalf("expected table output, got raw JSON %q", output)
	}
}
