package registry_test

import (
	"errors"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	assert "github.com/stretchr/testify/assert"
)

func Test_ProviderErr(t *testing.T) {
	// A provider which is busy is tried again in a moment, and says why, in
	// its words, rather than its response
	busy := registry.ProviderErr(httpresponse.Err(429).With(`Too Many Requests: 429 Too Many Requests: {"detail":{"type":"rate_limit_error","code":"concurrent_limit_exceeded","message":"Too many concurrent requests."}}`))
	assert.ErrorIs(t, busy, llm.ErrServiceUnavailable)
	assert.Contains(t, busy.Error(), "try again in a moment, one at a time")
	assert.Contains(t, busy.Error(), "Too many concurrent requests.")
	assert.NotContains(t, busy.Error(), "rate_limit_error")

	// One which refuses it won't work when it's tried again as it is
	refused := registry.ProviderErr(httpresponse.Err(403).With(`Forbidden: 403 Forbidden: {"detail":{"type":"authorization_error","code":"voice_access_denied","message":"Voice 'x' requires verification."}}`))
	assert.ErrorIs(t, refused, llm.ErrServiceUnavailable, "the provider's, rather than the caller's")
	assert.Contains(t, refused.Error(), "won't work when it's tried again")
	assert.Contains(t, refused.Error(), "Voice 'x' requires verification.")
	unauthorized := registry.ProviderErr(httpresponse.Err(401).With(`Unauthorized: {"error":{"message":"Incorrect API key"}}`))
	assert.ErrorIs(t, unauthorized, llm.ErrServiceUnavailable)
	assert.Contains(t, unauthorized.Error(), "Incorrect API key")
	credits := registry.ProviderErr(httpresponse.Err(402).With(`Payment Required: {"message":"quota_exceeded"}`))
	assert.ErrorIs(t, credits, llm.ErrServiceUnavailable)
	assert.Contains(t, credits.Error(), "run out of credits")
	assert.Contains(t, credits.Error(), "quota_exceeded")

	// A provider which says it's out of credits with 429, as OpenAI does,
	// isn't to be tried again in a moment
	quota := registry.ProviderErr(httpresponse.Err(429).With(`Too Many Requests: {"error":{"message":"You exceeded your current quota.","type":"insufficient_quota","code":"insufficient_quota"}}`))
	assert.Contains(t, quota.Error(), "run out of credits")
	assert.NotContains(t, quota.Error(), "try again in a moment")
	assert.Contains(t, quota.Error(), "You exceeded your current quota.")

	// A request which isn't valid, an llm error, and another error, are as
	// they were
	assert.ErrorIs(t, registry.ProviderErr(httpresponse.Err(400).With("Bad Request")), llm.ErrBadParameter)
	assert.ErrorIs(t, registry.ProviderErr(llm.ErrNotFound.With("model")), llm.ErrNotFound)
	assert.ErrorIs(t, registry.ProviderErr(errors.New("connection refused")), llm.ErrServiceUnavailable)
	assert.Contains(t, registry.ProviderErr(httpresponse.Err(429).With("Too Many Requests")).Error(), "Too Many Requests)", "without a body, the error")
}
