package httpclient

import (
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

func Test_WellKnown(t *testing.T) {
	// A well-known path is inserted between a URL's host and its path, then
	// after its path, and at its root
	assert.Equal(t, []string{
		"https://example.com/.well-known/oauth-protected-resource/v1/mcp",
		"https://example.com/v1/mcp/.well-known/oauth-protected-resource",
		"https://example.com/.well-known/oauth-protected-resource",
	}, wellKnown("https://example.com/v1/mcp?x=1", ResourceMetadataPath))
	assert.Equal(t, []string{"https://example.com/.well-known/oauth-authorization-server"}, wellKnown("https://example.com/", OAuthMetadataPath))
	assert.Nil(t, wellKnown("not a url", OAuthMetadataPath))
	assert.Equal(t, "https://example.com:8443", origin("https://example.com:8443/mcp"))
	assert.Empty(t, origin("/mcp"))
}
