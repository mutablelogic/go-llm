package homeassistant_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	homeassistant "github.com/mutablelogic/go-llm/toolkit/homeassistant"
	httpclient "github.com/mutablelogic/go-llm/toolkit/homeassistant/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// request is a request received by the fake Home Assistant server
type request struct {
	method string
	path   string
	body   map[string]any
}

// newTools returns the tools, for a fake Home Assistant server, and the
// requests the server received
func newTools(t *testing.T) (map[string]llm.Tool, *[]request) {
	t.Helper()
	var requests []request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(data, &body)
		requests = append(requests, request{method: r.Method, path: r.URL.Path, body: body})

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/states" && r.Method == http.MethodGet:
			w.Write([]byte(`[
				{"entity_id":"light.kitchen","state":"on","attributes":{"friendly_name":"Kitchen"}},
				{"entity_id":"sensor.temperature","state":"21.5","attributes":{"friendly_name":"Temperature","device_class":"temperature","unit_of_measurement":"°C"}},
				{"entity_id":"light.garden","state":"unavailable","attributes":{}}
			]`))
		case r.URL.Path == "/api/states/light.kitchen" && r.Method == http.MethodGet:
			w.Write([]byte(`{"entity_id":"light.kitchen","state":"on","attributes":{"friendly_name":"Kitchen"}}`))
		case strings.HasPrefix(r.URL.Path, "/api/states/") && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Entity not found."}`))
		case strings.HasPrefix(r.URL.Path, "/api/states/") && r.Method == http.MethodPost:
			state, _ := body["state"].(string)
			w.Write([]byte(`{"entity_id":"` + strings.TrimPrefix(r.URL.Path, "/api/states/") + `","state":"` + state + `"}`))
		case r.URL.Path == "/api/services" && r.Method == http.MethodGet:
			w.Write([]byte(`[{"domain":"light","services":{"turn_on":{"name":"Turn on","description":"Turn on a light","fields":{"entity_id":{},"brightness":{}}},"toggle":{"name":"Toggle"}}}]`))
		case strings.HasPrefix(r.URL.Path, "/api/services/") && r.Method == http.MethodPost:
			w.Write([]byte(`[{"entity_id":"light.kitchen","state":"on"}]`))
		case strings.HasPrefix(r.URL.Path, "/api/events/") && r.Method == http.MethodPost:
			w.Write([]byte(`{"message":"Event ` + strings.TrimPrefix(r.URL.Path, "/api/events/") + ` fired."}`))
		case r.URL.Path == "/api/template" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("  21.5\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	tools, err := homeassistant.NewTools(server.URL, "test-token")
	require.NoError(t, err)
	result := make(map[string]llm.Tool, len(tools))
	for _, tool := range tools {
		result[tool.Name()] = tool
	}
	return result, &requests
}

// encode returns the result of a tool as JSON, as the toolkit does
func encode(t *testing.T, result any) string {
	t.Helper()
	data, err := json.Marshal(result)
	require.NoError(t, err)
	return string(data)
}

func TestNewTools(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		_, err := homeassistant.NewTools("http://localhost:8123", "")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = homeassistant.NewTools("", "test-token")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Tools", func(t *testing.T) {
		tools, _ := newTools(t)
		for name, property := range map[string]string{
			"get_states":   "domain",
			"get_state":    "entity_id",
			"get_services": "domain",
			"call_service": "service",
			"set_state":    "state",
			"fire_event":   "event_type",
			"template":     "template",
		} {
			tool, ok := tools[name]
			require.True(t, ok, name)
			assert.True(t, llm.IsToolName(tool.Name()), name)
			assert.NotEmpty(t, tool.Description(), name)
			assert.Contains(t, tool.InputSchema().Properties, property, name)
		}
		assert.Len(t, tools, 7)
	})
}

func TestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("GetStates", func(t *testing.T) {
		// Unavailable entities are left out
		tools, _ := newTools(t)
		result, err := tools["get_states"].Run(ctx, nil)
		require.NoError(t, err)
		assert.JSONEq(t, `[
			{"entity_id":"light.kitchen","state":"on","name":"Kitchen","class":"light"},
			{"entity_id":"sensor.temperature","state":"21.5","name":"Temperature","class":"temperature","unit":"°C"}
		]`, encode(t, result))
	})

	t.Run("GetStatesDomain", func(t *testing.T) {
		tools, _ := newTools(t)
		result, err := tools["get_states"].Run(ctx, json.RawMessage(`{"domain":"sensor"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `[{"entity_id":"sensor.temperature","state":"21.5","name":"Temperature","class":"temperature","unit":"°C"}]`, encode(t, result))
	})

	t.Run("GetState", func(t *testing.T) {
		tools, _ := newTools(t)
		result, err := tools["get_state"].Run(ctx, json.RawMessage(`{"entity_id":"light.kitchen"}`))
		require.NoError(t, err)
		state, ok := result.(*httpclient.State)
		require.True(t, ok, "result is %T", result)
		assert.Equal(t, "on", state.State)

		// An unknown entity is an error
		_, err = tools["get_state"].Run(ctx, json.RawMessage(`{"entity_id":"light.other"}`))
		assert.Error(t, err)
	})

	t.Run("GetServices", func(t *testing.T) {
		tools, _ := newTools(t)
		result, err := tools["get_services"].Run(ctx, json.RawMessage(`{"domain":"light"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `[
			{"call":"toggle","name":"Toggle"},
			{"call":"turn_on","name":"Turn on","description":"Turn on a light","fields":["brightness","entity_id"]}
		]`, encode(t, result))

		_, err = tools["get_services"].Run(ctx, json.RawMessage(`{"domain":"unknown"}`))
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("CallService", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["call_service"].Run(ctx, json.RawMessage(`{"domain":"light","service":"turn_on","data":{"entity_id":"light.kitchen","brightness":255}}`))
		require.NoError(t, err)
		assert.JSONEq(t, `[{"entity_id":"light.kitchen","state":"on","last_changed":"0001-01-01T00:00:00Z","last_reported":"0001-01-01T00:00:00Z","last_updated":"0001-01-01T00:00:00Z","attributes":null,"context":{}}]`, encode(t, result))
		require.Len(t, *requests, 1)
		assert.Equal(t, "/api/services/light/turn_on", (*requests)[0].path)
		assert.Equal(t, map[string]any{"entity_id": "light.kitchen", "brightness": float64(255)}, (*requests)[0].body)
	})

	t.Run("SetState", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["set_state"].Run(ctx, json.RawMessage(`{"entity_id":"sensor.virtual","state":"42"}`))
		require.NoError(t, err)
		state, ok := result.(*httpclient.State)
		require.True(t, ok, "result is %T", result)
		assert.Equal(t, "42", state.State)
		assert.Equal(t, "/api/states/sensor.virtual", (*requests)[0].path)
	})

	t.Run("FireEvent", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["fire_event"].Run(ctx, json.RawMessage(`{"event_type":"my_event","event_data":{"a":1}}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"message":"Event my_event fired."}`, encode(t, result))
		assert.Equal(t, map[string]any{"a": float64(1)}, (*requests)[0].body)
	})

	t.Run("Template", func(t *testing.T) {
		// The rendered result is returned as it is, including its whitespace
		tools, _ := newTools(t)
		result, err := tools["template"].Run(ctx, json.RawMessage(`{"template":"{{ states(\"sensor.temperature\") }}"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"result":"  21.5\n"}`, encode(t, result))
	})

	t.Run("Invalid", func(t *testing.T) {
		tools, requests := newTools(t)
		for name, test := range map[string]struct {
			tool  string
			input string
		}{
			"GetStateMissing":    {"get_state", `{}`},
			"GetServicesMissing": {"get_services", ``},
			"CallMissingDomain":  {"call_service", `{"service":"turn_on"}`},
			"CallMissingService": {"call_service", `{"domain":"light"}`},
			"SetStateMissing":    {"set_state", `{"entity_id":"sensor.virtual"}`},
			"FireEventMissing":   {"fire_event", `{}`},
			"TemplateMissing":    {"template", `{"template":" "}`},
			"InvalidJSON":        {"get_states", `{`},
		} {
			result, err := tools[test.tool].Run(ctx, json.RawMessage(test.input))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
			assert.Nil(t, result, name)
		}

		// Invalid input isn't sent to Home Assistant
		assert.Empty(t, *requests)
	})

	t.Run("Cancelled", func(t *testing.T) {
		tools, requests := newTools(t)
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := tools["get_states"].Run(ctx, nil)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, *requests)
	})
}
