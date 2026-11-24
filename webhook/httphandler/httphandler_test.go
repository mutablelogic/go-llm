package httphandler_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	// Packages
	test "github.com/mutablelogic/go-llm/registry/test"
	httphandler "github.com/mutablelogic/go-llm/webhook/httphandler"
	registry "github.com/mutablelogic/go-llm/webhook/registry"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	store "github.com/mutablelogic/go-llm/webhook/store/sqlite"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	otel "github.com/mutablelogic/go-server/pkg/otel"
	types "github.com/mutablelogic/go-server/pkg/types"
	sqlite "github.com/mutablelogic/go-sqlite"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gootel "go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

const endpoint = "https://llm.example.com/api/"

func newServer(t *testing.T) *test.Server {
	t.Helper()
	s, err := store.New(context.Background(), sqlite.Memory)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	r, err := registry.New(s)
	require.NoError(t, err)
	return test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router, func() string { return endpoint })
	})
}

// hook calls the webhook's URL, as GitHub does, signed with the secret,
// unless it's empty, and returns its status, and body
func hook(t *testing.T, server *test.Server, method, token, body, secret, delivery string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+types.JoinPath(test.Prefix, httphandler.HookPath+"/"+token), strings.NewReader(body))
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-GitHub-Delivery", delivery)
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(body))
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, data
}

func Test_Webhooks(t *testing.T) {
	server := newServer(t)
	session := uuid.New()

	// Created, with its token, and its URL, of the endpoint
	var created schema.WebhookToken
	require.Equal(t, http.StatusCreated, server.JSON(t, http.MethodPost, "webhook", schema.WebhookMeta{Session: session, Description: "GitHub issues", Type: "github", Secret: "s3cret"}, &created))
	assert.NotZero(t, created.ID)
	assert.True(t, created.HasSecret)
	assert.Empty(t, created.Secret)
	assert.NotEmpty(t, created.Token)
	assert.Equal(t, "https://llm.example.com/api/hook/"+created.Token, created.URL)
	assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPost, "webhook", schema.WebhookMeta{}, nil))
	assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPost, "webhook", schema.WebhookMeta{Session: session, Description: "Unknown", Type: "unknown"}, nil))
	assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPost, "webhook", schema.WebhookMeta{Session: session, Description: " "}, nil))

	// Got, listed, and changed, without its secret, or token
	var webhook schema.Webhook
	id := fmt.Sprint(created.ID)
	require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "webhook/"+id, nil, &webhook))
	assert.Equal(t, created.Webhook, webhook)
	var list schema.WebhookList
	require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "webhook?session="+session.String(), nil, &list))
	assert.Equal(t, uint(1), list.Count)
	require.Equal(t, http.StatusOK, server.JSON(t, http.MethodPatch, "webhook/"+id, map[string]any{"description": "Issues"}, &webhook))
	assert.Equal(t, "Issues", webhook.Description)
	assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPatch, "webhook/"+id, map[string]any{"description": ""}, nil))
	assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodGet, "webhook/999", nil, nil))
	assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodGet, "webhook/abc", nil, nil))
	assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodGet, "webhook?session=abc", nil, nil))

	// Called, by its service: kept, a delivery sent again, and refused
	status, body := hook(t, server, http.MethodPost, created.Token, `{"action":"opened"}`, "s3cret", "d1")
	require.Equal(t, http.StatusAccepted, status, string(body))
	var accepted schema.HookResponse
	require.NoError(t, json.Unmarshal(body, &accepted))
	assert.NotZero(t, accepted.Call)
	status, body = hook(t, server, http.MethodPost, created.Token, `{"action":"opened"}`, "s3cret", "d1")
	require.Equal(t, http.StatusOK, status)
	require.NoError(t, json.Unmarshal(body, &accepted))
	assert.True(t, accepted.Duplicate)
	status, _ = hook(t, server, http.MethodPost, created.Token, `{"action":"closed"}`, "wrong", "d2")
	assert.Equal(t, http.StatusForbidden, status)
	status, _ = hook(t, server, http.MethodPost, "not-a-token", `{}`, "s3cret", "d3")
	assert.Equal(t, http.StatusNotFound, status)

	// Its calls, which were accepted, and refused
	var calls schema.CallList
	require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "webhook/call?webhook="+id, nil, &calls))
	require.Equal(t, uint(2), calls.Count)
	assert.Equal(t, "issues.opened", calls.Body[0].Event)
	assert.True(t, calls.Body[0].Verified)
	assert.NotEmpty(t, calls.Body[1].Error)
	require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, fmt.Sprintf("webhook/call?status=accepted&after=%d", 0), nil, &calls))
	assert.Equal(t, uint(1), calls.Count)
	require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, fmt.Sprintf("webhook/call?status=refused&session=%s", session), nil, &calls))
	assert.Equal(t, uint(1), calls.Count)
	assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodGet, "webhook/call?status=pending", nil, nil))
	var call schema.Call
	require.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, fmt.Sprintf("webhook/call/%d", accepted.Call), nil, &call))
	assert.Equal(t, "d1", call.Delivery)
	assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodGet, "webhook/call/999", nil, nil))

	// Deleted, so its URL stops working
	require.Equal(t, http.StatusOK, server.JSON(t, http.MethodDelete, "webhook/"+id, nil, &webhook))
	assert.Equal(t, created.ID, webhook.ID)
	status, _ = hook(t, server, http.MethodPost, created.Token, `{"action":"opened"}`, "s3cret", "d4")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodDelete, "webhook/"+id, nil, nil))
}

func Test_HookMethods(t *testing.T) {
	server := newServer(t)

	// A generic webhook, without a secret, is called with GET, POST or PUT
	var created schema.WebhookToken
	require.Equal(t, http.StatusCreated, server.JSON(t, http.MethodPost, "webhook", schema.WebhookMeta{Session: uuid.New(), Description: "Generic"}, &created))
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		status, body := hook(t, server, method, created.Token, "", "", "")
		assert.Equal(t, http.StatusAccepted, status, method+": "+string(body))
	}
}

func Test_HookURLWithoutEndpoint(t *testing.T) {
	s, err := store.New(context.Background(), sqlite.Memory)
	require.NoError(t, err)
	defer s.Close()
	r, err := registry.New(s)
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router, func() string { return "" })
	})

	// Without an endpoint, a webhook has its token, but no URL
	var created schema.WebhookToken
	require.Equal(t, http.StatusCreated, server.JSON(t, http.MethodPost, "webhook", schema.WebhookMeta{Session: uuid.New(), Description: "Generic"}, &created))
	assert.NotEmpty(t, created.Token)
	assert.Empty(t, created.URL)
}

func Test_HookTokenNotLogged(t *testing.T) {
	ctx := t.Context()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	gootel.SetTracerProvider(provider)
	t.Cleanup(func() { gootel.SetTracerProvider(tracenoop.NewTracerProvider()) })

	s, err := store.New(ctx, sqlite.Memory)
	require.NoError(t, err)
	defer s.Close()
	r, err := registry.New(s)
	require.NoError(t, err)
	var logs bytes.Buffer
	router, err := httprouter.NewRouter(ctx, http.NewServeMux(), "/api", "*", "test", "v1",
		otel.HTTPHandlerFunc("test", slog.New(slog.NewTextHandler(&logs, nil))))
	require.NoError(t, err)
	require.NoError(t, httphandler.RegisterHandlers(r, router, nil))
	generic, err := r.CreateWebhook(ctx, schema.WebhookMeta{Session: uuid.New(), Description: "Generic"})
	require.NoError(t, err)
	github, err := r.CreateWebhook(ctx, schema.WebhookMeta{Session: uuid.New(), Description: "GitHub", Type: "github", Secret: "s3cret"})
	require.NoError(t, err)

	// A webhook's token isn't in the log, nor the traces, whether its call is
	// kept, or refused, or its method isn't one, or no webhook has it
	for _, test := range []struct {
		name, method, token string
		status              int
	}{
		{"Kept", http.MethodPost, generic.Token, http.StatusAccepted},
		{"Refused", http.MethodPost, github.Token, http.StatusForbidden},
		{"Method not allowed", http.MethodDelete, generic.Token, http.StatusMethodNotAllowed},
		{"Not found", http.MethodPost, "another-token", http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			logs.Reset()
			exporter.Reset()
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(test.method, "/api/hook/"+test.token, nil))
			require.Equal(t, test.status, w.Code)
			assert.Contains(t, logs.String(), test.method+" /api/hook/{token}")
			assert.NotContains(t, logs.String(), test.token)
			require.NotEmpty(t, exporter.GetSpans())
			for _, span := range exporter.GetSpans() {
				assert.NotContains(t, span.Name, test.token)
				for _, attr := range span.Attributes {
					assert.NotContains(t, attr.Value.Emit(), test.token, string(attr.Key))
				}
			}
		})
	}
}
