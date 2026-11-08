package registry_test

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Templates(t *testing.T) {
	ctx := t.Context()

	t.Run("Embedded", func(t *testing.T) {
		// The servers which are offered by default are valid, and include
		// GitHub's
		templates, err := registry.Templates()
		require.NoError(t, err)
		require.NotEmpty(t, templates)
		var names []string
		for _, template := range templates {
			names = append(names, template.Name)
		}
		assert.Subset(t, names, []string{"GitHub", "Airtable", "Asana", "Atlassian", "Canva", "Cloudflare", "Datadog", "der die das (Lingviko)", "Grafana", "Harvey", "HubSpot", "Indeed", "Intercom", "Kensho kFinance", "Linear", "Miro", "Notion", "PagerDuty", "Pidgeon", "PostEverywhere", "Stripe", "Workable", "Zoom"})
	})

	t.Run("Offered", func(t *testing.T) {
		// Only those whose credentials the provider has are offered, in
		// order of their names, and one without credentials always is
		templates := []schema.ConnectorTemplate{
			{Name: "Zeta", URL: "https://zeta.example.com/mcp", ClientIDCredential: "ZETA_ID", ClientSecretCredential: "ZETA_SECRET"},
			{Name: "alpha", URL: "https://alpha.example.com/mcp"},
			{Name: "Missing", URL: "https://missing.example.com/mcp", ClientIDCredential: "MISSING_ID"},
			{Name: "Half", URL: "https://half.example.com/mcp", ClientIDCredential: "ZETA_ID", ClientSecretCredential: "HALF_SECRET"},
			{Name: "Empty", URL: "https://empty.example.com/mcp", ClientIDCredential: "EMPTY_ID", ClientSecretCredential: "ZETA_SECRET"},
		}
		r, err := registry.New(registry.WithTemplates(templates...), registry.WithCredentialsProvider(secrets{"ZETA_ID": "id", "ZETA_SECRET": "secret", "EMPTY_ID": ""}))
		require.NoError(t, err)
		list, err := r.ListTemplates(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint(2), list.Count)
		if assert.Len(t, list.Body, 2) {
			assert.Equal(t, "alpha", list.Body[0].Name)
			assert.Equal(t, "Zeta", list.Body[1].Name)
			assert.Equal(t, "ZETA_ID", list.Body[1].ClientIDCredential, "the names of its credentials, not the credentials")
		}

		// But not one whose server is a connector's already, with the same
		// URL, but for the case of its host, and a slash at its end, until
		// it's deleted
		_, err = r.CreateConnector(ctx, connector("zeta", "https://ZETA.example.com/mcp/"))
		require.NoError(t, err)
		list, err = r.ListTemplates(ctx)
		require.NoError(t, err)
		if assert.Equal(t, uint(1), list.Count) {
			assert.Equal(t, "alpha", list.Body[0].Name)
		}
		_, err = r.DeleteConnector(ctx, "zeta")
		require.NoError(t, err)
		list, err = r.ListTemplates(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint(2), list.Count)

		// And none are, with none
		r, err = registry.New(registry.WithTemplates())
		require.NoError(t, err)
		list, err = r.ListTemplates(ctx)
		require.NoError(t, err)
		assert.Zero(t, list.Count)
	})

	t.Run("EscapedPath", func(t *testing.T) {
		// A template whose path has an escaped slash isn't one whose path
		// has a slash, so a connector of one doesn't hide the other
		templates := []schema.ConnectorTemplate{
			{Name: "Escaped", URL: "https://example.com/tenant%2Fid/mcp"},
			{Name: "Plain", URL: "https://example.com/tenant/id/mcp"},
		}
		r, err := registry.New(registry.WithTemplates(templates...))
		require.NoError(t, err)
		_, err = r.CreateConnector(ctx, connector("plain", "https://example.com/tenant/id/mcp/"))
		require.NoError(t, err)
		list, err := r.ListTemplates(ctx)
		require.NoError(t, err)
		if assert.Equal(t, uint(1), list.Count) {
			assert.Equal(t, "Escaped", list.Body[0].Name)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		// A template without a name, or with a URL which isn't a remote
		// server's, isn't valid, as it's given, as it isn't in YAML
		for name, template := range map[string]schema.ConnectorTemplate{
			"URL":    {Name: "Local", URL: "stdio://server"},
			"NoName": {URL: "https://example.com/mcp"},
			"Spaces": {Name: "  ", URL: "https://example.com/mcp"},
		} {
			_, err := registry.New(registry.WithTemplates(template))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
		for name, yaml := range map[string]string{
			"NoName":  "- url: https://example.com/mcp\n",
			"Field":   "- name: X\n  url: https://example.com/mcp\n  client_secret: shh\n",
			"NotList": "name: X\n",
			"BadName": "- name: X\n  url: https://example.com/mcp\n  client_id_credential: 12345\n",
		} {
			_, err := registry.ParseTemplates([]byte(yaml))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
	})
}
