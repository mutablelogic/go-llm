package cmd

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

func TestCreateConnectorCommandRequest(t *testing.T) {
	assert := assert.New(t)
	enabled := false
	namespace := "mcp"

	req, err := (CreateConnectorCommand{
		URL: "https://example.com/sse",
		ConnectorMeta: schema.ConnectorMeta{
			Enabled:   &enabled,
			Namespace: &namespace,
			Groups:    []string{"admins"},
			Meta:      schema.ProviderMetaMap{"env": "dev"},
		},
	}).request()
	if !assert.NoError(err) {
		return
	}

	assert.Equal("https://example.com/sse", req.URL)
	if assert.NotNil(req.Enabled) {
		assert.False(*req.Enabled)
	}
	if assert.NotNil(req.Namespace) {
		assert.Equal("mcp", *req.Namespace)
	}
	assert.Equal([]string{"admins"}, req.Groups)
	assert.Equal("dev", req.Meta["env"])
}

func TestUpdateConnectorCommandRequest(t *testing.T) {
	assert := assert.New(t)
	enabled := false
	namespace := "mcp"

	req, err := (UpdateConnectorCommand{
		URL: "https://example.com/sse",
		ConnectorMeta: schema.ConnectorMeta{
			Enabled:   &enabled,
			Namespace: &namespace,
			Groups:    []string{"admins"},
			Meta:      schema.ProviderMetaMap{"env": "dev"},
		},
	}).request()
	if !assert.NoError(err) {
		return
	}

	if assert.NotNil(req.Enabled) {
		assert.False(*req.Enabled)
	}
	if assert.NotNil(req.Namespace) {
		assert.Equal("mcp", *req.Namespace)
	}
	assert.Equal([]string{"admins"}, req.Groups)
	assert.Equal("dev", req.Meta["env"])
}

func TestListConnectorsCommandEmbedsRequest(t *testing.T) {
	assert := assert.New(t)
	enabled := false
	cmd := ListConnectorsCommand{ConnectorListRequest: schema.ConnectorListRequest{Namespace: "mcp", Enabled: &enabled}}

	assert.Equal("mcp", cmd.Namespace)
	if assert.NotNil(cmd.Enabled) {
		assert.False(*cmd.Enabled)
	}
}

func TestCreateConnectorCommandRequestRejectsInvalidURL(t *testing.T) {
	assert := assert.New(t)

	_, err := (CreateConnectorCommand{URL: "sss"}).request()
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestGetConnectorCommandRejectsInvalidURL(t *testing.T) {
	assert := assert.New(t)

	_, err := schema.CanonicalURL((GetConnectorCommand{URL: "sss"}).URL)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestDeleteConnectorCommandRejectsInvalidURL(t *testing.T) {
	assert := assert.New(t)

	_, err := schema.CanonicalURL((DeleteConnectorCommand{URL: "sss"}).URL)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestUpdateConnectorCommandRejectsInvalidURL(t *testing.T) {
	assert := assert.New(t)

	_, err := (UpdateConnectorCommand{URL: "sss"}).request()
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}
