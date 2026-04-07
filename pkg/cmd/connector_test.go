package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	// Packages
	oidc "github.com/djthorpe/go-auth/pkg/oidc"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	assert "github.com/stretchr/testify/assert"
)

func TestCreateConnectorCommandUnauthorizedDetailDecode(t *testing.T) {
	assert := assert.New(t)

	errResponse := httpresponse.ErrResponse{
		Code:   401,
		Reason: "Unauthorized",
		Detail: schema.CreateConnectorUnauthorizedResponse{
			CodeFlow: &oidc.BaseConfiguration{Issuer: "https://mcp.asana.com"},
			Scopes:   []string{"default"},
		},
	}
	payload, err := json.Marshal(errResponse)
	if !assert.NoError(err) {
		return
	}

	err = fmt.Errorf("%w: 401 Unauthorized: %s", httpresponse.ErrNotAuthorized, payload)

	var httpErr httpresponse.Err
	if assert.True(errors.As(err, &httpErr)) {
		assert.Equal(httpresponse.ErrNotAuthorized, httpErr)
	}

	text := err.Error()
	index := strings.IndexByte(text, '{')
	if !assert.NotEqual(-1, index) {
		return
	}

	var response struct {
		Detail schema.CreateConnectorUnauthorizedResponse `json:"detail"`
	}
	if !assert.NoError(json.Unmarshal([]byte(text[index:]), &response)) {
		return
	}

	if assert.NotNil(response.Detail.CodeFlow) {
		assert.Equal("https://mcp.asana.com", response.Detail.CodeFlow.Issuer)
	}
	assert.Equal([]string{"default"}, response.Detail.Scopes)
}

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
