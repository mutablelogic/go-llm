package registry_test

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	test "github.com/mutablelogic/go-llm/mcp/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Authorize(t *testing.T) {
	setInterval(t, &registry.RetryInterval, time.Hour)
	setInterval(t, &registry.ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	server := test.NewServer(t, test.WithAuth())
	sdkmcp.AddTool(server.MCP, &sdkmcp.Tool{Name: "echo", Description: "Echo"}, func(context.Context, *sdkmcp.CallToolRequest, struct{}) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{}, nil, nil
	})
	b := broadcaster.New()
	ch := watch(t, b)
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	credentials := registry.NewMemoryCredentials()
	r, err := registry.New(registry.WithToolkit(tk), registry.WithBroadcaster(b), registry.WithCredentials(credentials), registry.WithName("test", "1.0"))
	require.NoError(t, err)
	run(t, r)

	// A connector whose server needs to be authorized says so, and its
	// event does
	_, err = r.CreateConnector(ctx, connector("secure", server.URL))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return status(t, r, "secure").Auth == schema.AuthRequired }, 5*time.Second, 5*time.Millisecond)
	assert.False(t, status(t, r, "secure").Connected)
	var required bool
	for _, event := range published(ch) {
		var data schema.ConnectorEvent
		if event.Action == eventschema.EventUpdate && event.Row == "secure" && json.Unmarshal(event.Data, &data) == nil && data.Auth == schema.AuthRequired {
			required = true
		}
	}
	assert.True(t, required, "its event says it needs to be authorized")

	// It's authorized, at the URL, as a person would, and its callback
	// connects it, with its token
	callback := "http://localhost:8084/api" + registry.CallbackPath
	authorizeURL, err := r.Authorize(ctx, "secure", callback)
	require.NoError(t, err)
	u, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	assert.Equal(t, "S256", u.Query().Get("code_challenge_method"))
	assert.Equal(t, server.URL, u.Query().Get("resource"))
	redirect, err := server.Approve(authorizeURL)
	require.NoError(t, err)
	to, err := url.Parse(redirect)
	require.NoError(t, err)
	assert.Equal(t, "/api"+registry.CallbackPath, to.Path)
	connected, err := r.Callback(ctx, to.Query().Get("state"), to.Query().Get("code"), "")
	require.NoError(t, err)
	assert.Equal(t, "secure", connected.Name)
	require.Eventually(t, func() bool { return status(t, r, "secure").Connected }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, schema.AuthAuthorized, status(t, r, "secure").Auth)
	require.Eventually(t, func() bool { return toolCount(t, tk, "secure") == 1 }, 5*time.Second, 5*time.Millisecond)
	credential, err := credentials.GetCredential(ctx, "secure")
	require.NoError(t, err)
	assert.NotEmpty(t, credential.ClientID)
	assert.Equal(t, []string{callback}, credential.RedirectURIs)
	require.NotNil(t, credential.Token)
	assert.NotEmpty(t, credential.Token.RefreshToken)

	// A state which isn't waited for, or once it's been used, isn't
	_, err = r.Callback(ctx, to.Query().Get("state"), to.Query().Get("code"), "")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	authorizeURL, err = r.Authorize(ctx, "secure", callback)
	require.NoError(t, err)
	state := mustQuery(t, authorizeURL, "state")
	_, err = r.Callback(ctx, state, "", "access_denied")
	assert.ErrorIs(t, err, llm.ErrBadParameter, "the authorization server said why it wasn't")
	_, err = r.Authorize(ctx, "secure", "not a url")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = r.Authorize(ctx, "missing", callback)
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// And it's authorized from another place, with another callback, as
	// well, which is registered
	other := "https://llm.example.ts.net/api" + registry.CallbackPath
	authorizeURL, err = r.Authorize(ctx, "secure", other)
	require.NoError(t, err)
	_, err = server.Approve(authorizeURL)
	require.NoError(t, err)

	// Once it's deleted, its credential is
	_, err = r.DeleteConnector(ctx, "secure")
	require.NoError(t, err)
	_, err = credentials.GetCredential(ctx, "secure")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

// toolCount returns how many tools the toolkit has, in the namespace
func toolCount(t *testing.T, tk *toolkit.Toolkit, namespace string) int {
	t.Helper()
	return len(toolNames(t, tk, namespace))
}

// mustQuery returns the URL's query parameter
func mustQuery(t *testing.T, raw, key string) string {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u.Query().Get(key)
}

func Test_AuthorizeClient(t *testing.T) {
	setInterval(t, &registry.RetryInterval, time.Hour)
	setInterval(t, &registry.ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	server := test.NewServer(t, test.WithAuth())
	callback := "http://localhost:8084/api" + registry.CallbackPath
	server.Client("fixed", "s3cret", callback)
	credentials := registry.NewMemoryCredentials()
	provider := secrets{}
	r, err := registry.New(registry.WithCredentials(credentials), registry.WithCredentialsProvider(provider))
	require.NoError(t, err)
	run(t, r)

	// A connector with its own client, whose identifier and secret are
	// placeholders, isn't valid while its credentials don't exist
	c := connector("fixed", server.URL)
	c.ClientID, c.ClientSecret = "${CLIENT_ID}", "${CLIENT_SECRET}"
	_, err = r.CreateConnector(ctx, c)
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Once they do, it's created, with its placeholders, rather than its
	// credentials, and without its secret
	provider["CLIENT_ID"], provider["CLIENT_SECRET"] = "fixed", "s3cret"
	created, err := r.CreateConnector(ctx, c)
	require.NoError(t, err)
	assert.Equal(t, "${CLIENT_ID}", created.ClientID)
	assert.Empty(t, created.ClientSecret, "its secret isn't returned")
	require.Eventually(t, func() bool { return status(t, r, "fixed").Auth == schema.AuthRequired }, 5*time.Second, 5*time.Millisecond)

	// And it's authorized as its client, rather than one which is
	// registered, with its secret
	authorizeURL, err := r.Authorize(ctx, "fixed", callback)
	require.NoError(t, err)
	assert.Equal(t, "fixed", mustQuery(t, authorizeURL, "client_id"))
	redirect, err := server.Approve(authorizeURL)
	require.NoError(t, err)
	_, err = r.Callback(ctx, mustQuery(t, redirect, "state"), mustQuery(t, redirect, "code"), "")
	require.NoError(t, err)
	require.Eventually(t, func() bool { return status(t, r, "fixed").Connected }, 5*time.Second, 5*time.Millisecond)
	credential, err := credentials.GetCredential(ctx, "fixed")
	require.NoError(t, err)
	assert.Equal(t, "fixed", credential.ClientID)
	assert.Empty(t, credential.ClientSecret, "its secret isn't stored")

	// Its secret is kept when it's updated, without one, as it isn't
	// returned, so can't be sent back
	meta := c.ConnectorMeta
	meta.ClientSecret = ""
	meta.Include = []string{"echo"}
	_, err = r.UpdateConnector(ctx, "fixed", meta)
	require.NoError(t, err)
	authorizeURL, err = r.Authorize(ctx, "fixed", callback)
	require.NoError(t, err)
	redirect, err = server.Approve(authorizeURL)
	require.NoError(t, err)
	_, err = r.Callback(ctx, mustQuery(t, redirect, "state"), mustQuery(t, redirect, "code"), "")
	require.NoError(t, err, "its secret was kept")
}

// secrets is a credentials provider, whose credentials are its own
type secrets map[string]string

func (s secrets) Expand(_ context.Context, value string) (string, error) {
	var err error
	result := os.Expand(value, func(key string) string {
		v, ok := s[key]
		if !ok {
			err = llm.ErrNotFound.Withf("credential %q isn't set", key)
		}
		return v
	})
	return result, err
}
