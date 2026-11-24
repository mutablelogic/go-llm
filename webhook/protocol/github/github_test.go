package github_test

import (
	"net/http"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	github "github.com/mutablelogic/go-llm/webhook/protocol/github"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_GitHub(t *testing.T) {
	p := github.New()

	// GitHub's own example, from its docs
	secret, body := "It's a Secret to Everybody", []byte("Hello, World!")
	signed := http.Header{"X-Hub-Signature-256": {"sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17"}}
	assert.NoError(t, p.Verify(signed, body, secret))
	assert.ErrorIs(t, p.Verify(signed, []byte("Hello, World?"), secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(signed, body, "another"), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(http.Header{"X-Hub-Signature-256": {"sha256=zz"}}, body, secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(http.Header{}, body, secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(signed, body, ""), llm.ErrForbidden)

	// A form's payload is the JSON
	values, err := p.Decode(http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}, []byte(`payload=%7B%22action%22%3A%22opened%22%7D`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"action":"opened"}`, string(values))

	// The event, with its action, and the delivery
	h := http.Header{}
	h.Set("X-GitHub-Event", "issues")
	h.Set("X-GitHub-Delivery", "72d3162e")
	event, delivery := p.Identify(h, values)
	assert.Equal(t, "issues.opened", event)
	assert.Equal(t, "72d3162e", delivery)
	h.Set("X-GitHub-Event", "push")
	event, _ = p.Identify(h, []byte(`{"ref":"main"}`))
	assert.Equal(t, "push", event)
}
