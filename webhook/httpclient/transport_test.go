package httpclient_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	transport "github.com/mutablelogic/go-client/pkg/transport"
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func Test_HookTokenNotTraced(t *testing.T) {
	const token = "s3cretTOKENvalue"
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch {
		case strings.HasPrefix(r.URL.Path, "/old/"):
			http.Redirect(w, r, "/api"+strings.TrimPrefix(r.URL.Path, "/old"), http.StatusTemporaryRedirect)
		case r.Method == http.MethodPut:
			// An error which has the path, as go-server's not found has
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":403,"reason":"Forbidden","detail":"` + r.RequestURI + `"}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"call":1}`))
		}
	}))
	defer server.Close()

	for _, test := range []struct {
		name, prefix, method string
		paths                []string
		fails                bool
	}{
		{"Accepted", "/api", http.MethodPost, []string{"/api/hook/" + token}, false},
		{"Refused", "/api", http.MethodPut, []string{"/api/hook/" + token}, true},
		{"Redirected", "/old", http.MethodGet, []string{"/old/hook/" + token, "/api/hook/" + token}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			paths = nil
			recorder := tracetest.NewSpanRecorder()
			tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
			var logs bytes.Buffer
			c, err := httpclient.New(server.URL+test.prefix, client.OptTracer(tracer), client.OptTransport(func(base http.RoundTripper) http.RoundTripper {
				return transport.NewLogging(&logs, base, true)
			}))
			require.NoError(t, err)

			// The server has the token, which isn't in the client's traces,
			// its log, or its error
			_, err = c.Hook(t.Context(), token, test.method, types.ContentTypeJSON, nil, nil)
			if test.fails {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), token)
				var response httpresponse.ErrResponse
				require.ErrorAs(t, err, &response)
				assert.Equal(t, http.StatusForbidden, response.Code)
				assert.Equal(t, "/api/hook/{token}", response.Detail)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, test.paths, paths)
			assert.NotEmpty(t, logs.String())
			assert.NotContains(t, logs.String(), token)
			require.Len(t, recorder.Ended(), len(test.paths))
			for _, span := range recorder.Ended() {
				assert.NotContains(t, span.Name(), token)
				for _, attr := range span.Attributes() {
					assert.NotContains(t, attr.Value.Emit(), token, string(attr.Key))
				}
			}
		})
	}

	// Nor in the error of a server which can't be reached
	server.Close()
	c, err := httpclient.New(server.URL)
	require.NoError(t, err)
	_, err = c.Hook(t.Context(), token, http.MethodPost, types.ContentTypeJSON, nil, nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), token)
	_, err = c.Hook(t.Context(), " ", http.MethodPost, types.ContentTypeJSON, nil, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
