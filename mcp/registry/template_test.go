package registry_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	test "github.com/mutablelogic/go-llm/mcp/test"
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

		// And each has an icon, which is among the web UI's
		for _, template := range templates {
			if assert.NotEmpty(t, template.Icon, template.Name) {
				_, err := os.Stat(filepath.Join("..", "..", "frontend", "assets", "dist", "icon", "connector", template.Icon))
				assert.NoError(t, err, "the icon of %q", template.Name)
			}
		}
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
		r, err := registry.New(registry.WithTemplates(templates...), registry.WithCredentialsProvider(newSecrets(map[string]string{"ZETA_ID": "id", "ZETA_SECRET": "secret", "EMPTY_ID": ""})))
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
			"URL":        {Name: "Local", URL: "stdio://server"},
			"NoName":     {URL: "https://example.com/mcp"},
			"Spaces":     {Name: "  ", URL: "https://example.com/mcp"},
			"IconPath":   {Name: "X", URL: "https://example.com/mcp", Icon: "../secret.svg"},
			"IconURL":    {Name: "X", URL: "https://example.com/mcp", Icon: "https://example.com/x.svg"},
			"IconFormat": {Name: "X", URL: "https://example.com/mcp", Icon: "x.gif"},
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

func Test_ConnectorIcon(t *testing.T) {
	setInterval(t, &registry.RetryInterval, time.Hour)
	setInterval(t, &registry.ConnectJitter, 10*time.Millisecond)
	ctx := t.Context()
	logo := "data:image/svg+xml;base64,PHN2Zy8+"
	dark := "data:image/png;base64,iVBORw0KGgo="
	plain := test.NewServer(t)
	icons := test.NewServer(t, test.WithIcons(
		sdkmcp.Icon{Source: "https://example.com/logo.svg"},
		sdkmcp.Icon{Source: dark, Theme: "dark"},
		sdkmcp.Icon{Source: logo, Theme: "light"},
	))
	elsewhere := test.NewServer(t, test.WithIcons(sdkmcp.Icon{Source: "https://example.com/logo.svg"}))
	r, err := registry.New(registry.WithTemplates(schema.ConnectorTemplate{Name: "Plain", URL: plain.URL + "/", Icon: "plain.svg"}))
	require.NoError(t, err)
	run(t, r)
	for name, url := range map[string]string{"plain": plain.URL, "icons": icons.URL, "elsewhere": elsewhere.URL} {
		_, err := r.CreateConnector(ctx, connector(name, url))
		require.NoError(t, err)
	}
	for _, name := range []string{"plain", "icons", "elsewhere"} {
		require.Eventually(t, func() bool { return status(t, r, name).Connected }, 5*time.Second, 5*time.Millisecond, name)
	}

	// A well-known server's is the web UI's, by its URL, whether it's
	// connected or not, and another's is the data URL its server sends,
	// for a light background, rather than one elsewhere, which isn't fetched
	assert.Equal(t, registry.TemplateIconPath+"plain.svg", status(t, r, "plain").Icon)
	assert.Equal(t, logo, status(t, r, "icons").Icon)
	assert.Empty(t, status(t, r, "elsewhere").Icon)

	// And a connector's definition has none
	got, err := r.GetConnector(ctx, "icons")
	require.NoError(t, err)
	assert.Empty(t, got.Definition().Icon)
}
