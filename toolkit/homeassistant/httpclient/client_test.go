package httpclient_test

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
	httpclient "github.com/mutablelogic/go-llm/toolkit/homeassistant/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// request is a request received by the fake Home Assistant server
type request struct {
	method string
	path   string
	query  string
	body   map[string]any
}

// newServer returns a fake Home Assistant server, and the requests it received
func newServer(t *testing.T) (*httptest.Server, *[]request) {
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
		requests = append(requests, request{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body})

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/":
			w.Write([]byte(`{"message":"API running."}`))
		case r.URL.Path == "/api/config":
			w.Write([]byte(`{"location_name":"Home","version":"2026.10.0","time_zone":"Europe/London"}`))
		case r.URL.Path == "/api/states" && r.Method == http.MethodGet:
			w.Write([]byte(`[
				{"entity_id":"light.kitchen","state":"on","attributes":{"friendly_name":"Kitchen"}},
				{"entity_id":"sensor.temperature","state":"21.5","attributes":{"friendly_name":"Temperature","device_class":"temperature","unit_of_measurement":"°C"}},
				{"entity_id":"light.garden","state":"unavailable","attributes":{}}
			]`))
		case r.URL.Path == "/api/states/light.kitchen" && r.Method == http.MethodGet:
			w.Write([]byte(`{"entity_id":"light.kitchen","state":"on","attributes":{"friendly_name":"Kitchen","brightness":255}}`))
		case strings.HasPrefix(r.URL.Path, "/api/states/sensor.") && r.Method == http.MethodPost:
			state, _ := body["state"].(string)
			w.Write([]byte(`{"entity_id":"` + strings.TrimPrefix(r.URL.Path, "/api/states/") + `","state":"` + state + `"}`))
		case strings.HasPrefix(r.URL.Path, "/api/states/") && r.Method == http.MethodDelete:
			w.Write([]byte(`{"message":"Entity removed."}`))
		case r.URL.Path == "/api/services" && r.Method == http.MethodGet:
			w.Write([]byte(`[{"domain":"light","services":{"turn_on":{"name":"Turn on","description":"Turn on a light","fields":{"brightness":{},"entity_id":{}}},"toggle":{"name":"Toggle"}}}]`))
		case r.URL.Path == "/api/services/light/turn_on" && r.Method == http.MethodPost:
			if r.URL.Query().Has("return_response") {
				w.Write([]byte(`{"changed_states":[{"entity_id":"light.kitchen","state":"on"}],"service_response":{"ok":true}}`))
			} else {
				w.Write([]byte(`[{"entity_id":"light.kitchen","state":"on"}]`))
			}
		case r.URL.Path == "/api/events" && r.Method == http.MethodGet:
			w.Write([]byte(`[{"event":"state_changed","listener_count":5}]`))
		case strings.HasPrefix(r.URL.Path, "/api/events/") && r.Method == http.MethodPost:
			w.Write([]byte(`{"message":"Event ` + strings.TrimPrefix(r.URL.Path, "/api/events/") + ` fired."}`))
		case r.URL.Path == "/api/template" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("21.5\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func newClient(t *testing.T) (*httpclient.Client, *[]request) {
	t.Helper()
	server, requests := newServer(t)
	client, err := httpclient.New(server.URL, "test-token")
	require.NoError(t, err)
	return client, requests
}

func TestNew(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		for name, test := range map[string][2]string{
			"MissingKey":      {"http://localhost:8123", ""},
			"MissingEndpoint": {"", "test-token"},
			"NoScheme":        {"localhost:8123", "test-token"},
		} {
			_, err := httpclient.New(test[0], test[1])
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
	})

	t.Run("Endpoint", func(t *testing.T) {
		// The server URL, or its API URL, can be used
		server, _ := newServer(t)
		for _, endpoint := range []string{server.URL, server.URL + "/", server.URL + "/api", server.URL + "/api/"} {
			client, err := httpclient.New(endpoint, "test-token")
			require.NoError(t, err, endpoint)
			message, err := client.Health(context.Background())
			require.NoError(t, err, endpoint)
			assert.Equal(t, "API running.", message, endpoint)
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		server, _ := newServer(t)
		client, err := httpclient.New(server.URL, "wrong-token")
		require.NoError(t, err)
		_, err = client.Health(context.Background())
		assert.Error(t, err)
	})
}

func TestClient(t *testing.T) {
	ctx := context.Background()

	t.Run("Config", func(t *testing.T) {
		client, _ := newClient(t)
		config, err := client.Config(ctx)
		require.NoError(t, err)
		assert.Equal(t, "Home", config.LocationName)
		assert.Equal(t, "2026.10.0", config.Version)
	})

	t.Run("States", func(t *testing.T) {
		client, _ := newClient(t)
		states, err := client.States(ctx)
		require.NoError(t, err)
		require.Len(t, states, 3)
		assert.Equal(t, "light.kitchen", states[0].Entity)
	})

	t.Run("State", func(t *testing.T) {
		client, requests := newClient(t)
		state, err := client.State(ctx, "light.kitchen")
		require.NoError(t, err)
		assert.Equal(t, "on", state.State)
		assert.Equal(t, float64(255), state.Attributes["brightness"])
		assert.Equal(t, "/api/states/light.kitchen", (*requests)[0].path)
	})

	t.Run("SetState", func(t *testing.T) {
		client, requests := newClient(t)
		state, err := client.SetState(ctx, "sensor.virtual", "42", map[string]any{"unit_of_measurement": "W"})
		require.NoError(t, err)
		assert.Equal(t, "42", state.State)
		require.Len(t, *requests, 1)
		assert.Equal(t, http.MethodPost, (*requests)[0].method)
		assert.Equal(t, map[string]any{"state": "42", "attributes": map[string]any{"unit_of_measurement": "W"}}, (*requests)[0].body)
	})

	t.Run("DeleteState", func(t *testing.T) {
		client, requests := newClient(t)
		require.NoError(t, client.DeleteState(ctx, "sensor.virtual"))
		assert.Equal(t, http.MethodDelete, (*requests)[0].method)
	})

	t.Run("Services", func(t *testing.T) {
		// The services are in order of name
		client, _ := newClient(t)
		services, err := client.Services(ctx, "light")
		require.NoError(t, err)
		require.Len(t, services, 2)
		assert.Equal(t, "toggle", services[0].Call)
		assert.Equal(t, "turn_on", services[1].Call)
		assert.Equal(t, "Turn on a light", services[1].Description)

		_, err = client.Services(ctx, "unknown")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Call", func(t *testing.T) {
		client, requests := newClient(t)
		states, err := client.Call(ctx, "light", "turn_on", map[string]any{"entity_id": "light.kitchen"})
		require.NoError(t, err)
		require.Len(t, states, 1)
		assert.Equal(t, "light.kitchen", states[0].Entity)
		assert.Equal(t, map[string]any{"entity_id": "light.kitchen"}, (*requests)[0].body)

		_, err = client.Call(ctx, "", "turn_on", nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = client.Call(ctx, "light", "", nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("CallWithResponse", func(t *testing.T) {
		client, requests := newClient(t)
		response, err := client.CallWithResponse(ctx, "light", "turn_on", nil)
		require.NoError(t, err)
		require.Len(t, response.ChangedStates, 1)
		assert.Equal(t, true, response.ServiceResponse["ok"])
		assert.Equal(t, "return_response=", (*requests)[0].query)
	})

	t.Run("Events", func(t *testing.T) {
		client, _ := newClient(t)
		events, err := client.Events(ctx)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, uint(5), events[0].Listeners)
	})

	t.Run("FireEvent", func(t *testing.T) {
		client, requests := newClient(t)
		message, err := client.FireEvent(ctx, "my_event", map[string]any{"a": 1})
		require.NoError(t, err)
		assert.Equal(t, "Event my_event fired.", message)
		assert.Equal(t, map[string]any{"a": float64(1)}, (*requests)[0].body)
	})

	t.Run("Template", func(t *testing.T) {
		client, requests := newClient(t)
		result, err := client.Template(ctx, `{{ states("sensor.temperature") }}`)
		require.NoError(t, err)
		assert.Equal(t, "21.5\n", result)
		assert.Equal(t, map[string]any{"template": `{{ states("sensor.temperature") }}`}, (*requests)[0].body)
	})
}

func TestState(t *testing.T) {
	sensor := httpclient.State{Entity: "sensor.temperature", State: "21.5", Attributes: map[string]any{
		"friendly_name": "Temperature", "device_class": "temperature", "unit_of_measurement": "°C",
	}}
	assert.Equal(t, "sensor", sensor.Domain())
	assert.Equal(t, "Temperature", sensor.Name())
	assert.Equal(t, "21.5", sensor.Value())
	assert.Equal(t, "temperature", sensor.Class())
	assert.Equal(t, "°C", sensor.UnitOfMeasurement())

	// Without attributes, the name is the entity, and the class is the domain
	light := httpclient.State{Entity: "light.kitchen", State: "unavailable"}
	assert.Equal(t, "light.kitchen", light.Name())
	assert.Equal(t, "", light.Value())
	assert.Equal(t, "light", light.Class())
	assert.Equal(t, "", light.UnitOfMeasurement())

	// An entity without a domain
	assert.Equal(t, "", httpclient.State{Entity: "nodomain"}.Domain())
}
