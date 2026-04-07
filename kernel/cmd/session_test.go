package cmd

import (
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	assert "github.com/stretchr/testify/assert"
)

func TestResolveSessionIDPrefersExplicitID(t *testing.T) {
	assert := assert.New(t)
	explicit := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	stored := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	id, err := resolveSessionID(explicit, stored.String())
	if !assert.NoError(err) {
		return
	}

	assert.Equal(explicit, id)
}

func TestResolveSessionIDUsesStoredDefault(t *testing.T) {
	assert := assert.New(t)
	stored := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	id, err := resolveSessionID(uuid.Nil, stored.String())
	if !assert.NoError(err) {
		return
	}

	assert.Equal(stored, id)
}

func TestResolveSessionIDRejectsMissingSession(t *testing.T) {
	assert := assert.New(t)

	_, err := resolveSessionID(uuid.Nil, "")
	if assert.Error(err) {
		assert.Contains(err.Error(), "session is required")
	}
}

func TestResolveSessionIDRejectsInvalidStoredSession(t *testing.T) {
	assert := assert.New(t)

	_, err := resolveSessionID(uuid.Nil, "not-a-uuid")
	if assert.Error(err) {
		assert.Contains(err.Error(), "invalid stored session")
	}
}
