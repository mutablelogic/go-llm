package protocol_test

import (
	"net/http"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	protocol "github.com/mutablelogic/go-llm/webhook/protocol"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Protocol_New(t *testing.T) {
	assert.Equal(t, []string{"aftership", "generic", "github", "pidgeon"}, protocol.Names())
	for name, want := range map[string]string{"": "generic", " GitHub ": "github", "aftership": "aftership", "pidgeon": "pidgeon"} {
		p, err := protocol.New(name)
		require.NoError(t, err)
		assert.Equal(t, want, p.Name())
	}
	_, err := protocol.New("unknown")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Protocol_Sample(t *testing.T) {
	// A sample of each protocol is a call which its service makes, which is
	// verified with the secret it's signed with, and not with another, or
	// when it isn't signed
	for _, name := range protocol.Names() {
		p, err := protocol.New(name)
		require.NoError(t, err)
		header, body := p.Sample("d1")
		assert.NoError(t, p.Check(http.MethodPost, header), name)
		values, err := p.Decode(header, body)
		require.NoError(t, err, name)
		assert.Contains(t, string(values), `"test":true`, name)
		if event, delivery := p.Identify(header, values); name != "generic" {
			assert.NotEmpty(t, event, name)
			assert.Equal(t, "d1", delivery, name)
		}
		assert.ErrorIs(t, p.Verify(header, body, "s3cret"), llm.ErrForbidden, name)
		p.Sign(header, body, "s3cret")
		assert.NoError(t, p.Verify(header, body, "s3cret"), name)
		assert.ErrorIs(t, p.Verify(header, body, "another"), llm.ErrForbidden, name)
		if name != "generic" {
			assert.ErrorIs(t, p.Verify(header, append(body, ' '), "s3cret"), llm.ErrForbidden, name)
		}
	}
}

func Test_Protocol_KeptHeaders(t *testing.T) {
	p, _ := protocol.New("github")
	h := http.Header{}
	for k, v := range map[string]string{"Content-Type": "application/json", "User-Agent": "GitHub-Hookshot/1", "X-GitHub-Event": "issues", "Cookie": "secret", "Authorization": "Bearer x"} {
		h.Set(k, v)
	}
	assert.Equal(t, map[string]string{"Content-Type": "application/json", "User-Agent": "GitHub-Hookshot/1", "X-Github-Event": "issues"}, protocol.KeptHeaders(p, h))
	assert.Nil(t, protocol.KeptHeaders(p, http.Header{}))
}
