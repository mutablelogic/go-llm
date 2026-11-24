package generic_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	generic "github.com/mutablelogic/go-llm/webhook/protocol/generic"
	assert "github.com/stretchr/testify/assert"
)

func Test_Generic(t *testing.T) {
	p := generic.New()
	bearer := func(token string) http.Header { return http.Header{"Authorization": {"Bearer " + token}} }
	assert.NoError(t, p.Verify(bearer("s3cret"), nil, "s3cret"))
	for _, value := range []string{"bearer s3cret", "BEARER s3cret", "Bearer   s3cret "} {
		assert.NoError(t, p.Verify(http.Header{"Authorization": {value}}, nil, "s3cret"), value)
	}
	for _, value := range []string{"s3cret", "Basic s3cret", "Bearer S3CRET", "Bearers3cret", "Bearer"} {
		assert.ErrorIs(t, p.Verify(http.Header{"Authorization": {value}}, nil, "s3cret"), llm.ErrForbidden, value)
	}
	assert.ErrorIs(t, p.Verify(bearer("wrong"), nil, "s3cret"), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(http.Header{}, nil, "s3cret"), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(bearer(""), nil, ""), llm.ErrForbidden)
	event, delivery := p.Identify(http.Header{}, nil)
	assert.Empty(t, event)
	assert.Empty(t, delivery)
	_, ok := p.Handshake(httptest.NewRequest(http.MethodGet, "/?challenge=x", nil), "")
	assert.False(t, ok)
}
