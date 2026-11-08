package registry_test

import (
	"fmt"
	"net/url"
	"testing"

	// Packages
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// liveServers are public MCP servers which need to be authorized, whose
// authorization is started, up to the URL a person logs in at, which
// registers a client with each, unless its authorization server doesn't
// register clients, when it's given one, which isn't real, and the name
// its connector is given, from what the server says, or its host, when
// it says nothing
var liveServers = []struct {
	url, clientID, name string
}{
	{"https://mcp.atlassian.com/v1/mcp/authv2", "", "mcp-atlassian"},
	{"https://bindings.mcp.cloudflare.com/mcp", "", "bindings-mcp-cloudflare"},
	{"https://api.githubcopilot.com/mcp/", "Iv1.go-llm-test", "github-mcp-server"},
	{"https://mcp.asana.com/v2/mcp", "go-llm-test", "asana-mcp"},
	{"https://mcp.notion.com/mcp", "", "notion-mcp-beta"},
	{"https://mcp.airtable.com/mcp", "", "mcp-airtable"},
	{"https://mcp.canva.com/mcp", "", "mcp-canva"},
	{"https://mcp.datadoghq.com/v1/mcp", "", "mcp-datadoghq"},
	{"https://mcp.grafana.com/mcp", "", "mcp-grafana"},
}

func Test_AuthorizeLive(t *testing.T) {
	// The authorization of each server is started, up to the URL a person
	// logs in at, with PKCE's S256, and its resource, unless -short
	if testing.Short() {
		t.Skip("calls public MCP servers, and registers clients with them")
	}
	ctx := t.Context()
	provider := secrets{"LIVE_CLIENT_ID": ""}
	r, err := registry.New(registry.WithName("go-llm-test", ""), registry.WithCredentialsProvider(provider))
	require.NoError(t, err)
	for i, server := range liveServers {
		t.Run(server.url, func(t *testing.T) {
			name := fmt.Sprintf("live%d", i)
			c := connector(name, server.url)
			if server.clientID != "" {
				provider["LIVE_CLIENT_ID"] = server.clientID
				c.ClientIDCredential = "LIVE_CLIENT_ID"
			}
			_, err := r.CreateConnector(ctx, c)
			require.NoError(t, err)
			authorizeURL, err := r.Authorize(ctx, name, "http://localhost:8084/api"+registry.CallbackPath)
			require.NoError(t, err)
			u, err := url.Parse(authorizeURL)
			require.NoError(t, err)
			t.Logf("%s => %s://%s%s", server.url, u.Scheme, u.Host, u.Path)
			q := u.Query()
			assert.Equal(t, "code", q.Get("response_type"))
			assert.Equal(t, "S256", q.Get("code_challenge_method"))
			if server.clientID != "" {
				assert.Equal(t, server.clientID, q.Get("client_id"))
			} else {
				assert.NotEmpty(t, q.Get("client_id"))
			}
			assert.NotEmpty(t, q.Get("state"))
			assert.NotEmpty(t, q.Get("resource"))
		})
	}
}

func Test_CreateNamedLive(t *testing.T) {
	// Each server's connector is named after it, as the metadata of how
	// it's authorized says, as it needs to be, unless -short
	if testing.Short() {
		t.Skip("calls public MCP servers")
	}
	ctx := t.Context()
	r, err := registry.New(registry.WithName("go-llm-test", ""))
	require.NoError(t, err)
	for _, server := range liveServers {
		t.Run(server.url, func(t *testing.T) {
			created, err := r.CreateConnector(ctx, connector("", server.url))
			require.NoError(t, err)
			t.Logf("%s => %s", server.url, created.Name)
			assert.Equal(t, server.name, created.Name)
		})
	}
}
