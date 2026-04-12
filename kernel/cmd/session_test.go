package cmd

import (
	"fmt"
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	assert "github.com/stretchr/testify/assert"
)

type sessionSetterStub struct {
	values map[string]string
	err    error
}

func (s *sessionSetterStub) Set(key string, value any) error {
	if s.err != nil {
		return s.err
	}
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[key] = fmt.Sprint(value)
	return nil
}

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

func TestMaybeStoreDefaultSessionStoresSession(t *testing.T) {
	assert := assert.New(t)
	setter := &sessionSetterStub{}
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	err := maybeStoreDefaultSession(setter, id, true)
	if !assert.NoError(err) {
		return
	}

	assert.Equal(id.String(), setter.values["session"])
}

func TestMaybeStoreDefaultSessionSkipsWhenNotDefault(t *testing.T) {
	assert := assert.New(t)
	setter := &sessionSetterStub{}
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	err := maybeStoreDefaultSession(setter, id, false)
	if !assert.NoError(err) {
		return
	}

	assert.Empty(setter.values)
}

func TestMaybeStoreDefaultSessionReturnsSetError(t *testing.T) {
	assert := assert.New(t)
	setter := &sessionSetterStub{err: fmt.Errorf("boom")}
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	err := maybeStoreDefaultSession(setter, id, true)
	if assert.Error(err) {
		assert.Contains(err.Error(), "boom")
	}
}
