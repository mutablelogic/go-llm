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

func Test_Protocol_KeptHeaders(t *testing.T) {
	p, _ := protocol.New("github")
	h := http.Header{}
	for k, v := range map[string]string{"Content-Type": "application/json", "User-Agent": "GitHub-Hookshot/1", "X-GitHub-Event": "issues", "Cookie": "secret", "Authorization": "Bearer x"} {
		h.Set(k, v)
	}
	assert.Equal(t, map[string]string{"Content-Type": "application/json", "User-Agent": "GitHub-Hookshot/1", "X-Github-Event": "issues"}, protocol.KeptHeaders(p, h))
	assert.Nil(t, protocol.KeptHeaders(p, http.Header{}))
}
