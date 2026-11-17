package registry

import (
	"errors"
	"testing"

	// Packages
	goauth "github.com/mutablelogic/go-auth"
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

func Test_LLMError(t *testing.T) {
	// go-auth's errors are llm's of their kind, with their reason
	err := llmError(goauth.ErrNotImplemented.Withf("authorization server %q doesn't register clients", "https://example.com"))
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
	assert.Equal(t, `not implemented: authorization server "https://example.com" doesn't register clients`, err.Error())
	assert.ErrorIs(t, llmError(goauth.ErrServiceUnavailable.With("register")), llm.ErrServiceUnavailable)
	assert.ErrorIs(t, llmError(goauth.ErrBadParameter.With("missing redirect URIs")), llm.ErrBadParameter)

	// Others are as they are
	other := errors.New("other")
	assert.Equal(t, other, llmError(other))
}
