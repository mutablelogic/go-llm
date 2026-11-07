package registry_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_SessionRetention(t *testing.T) {
	ctx := context.Background()
	sessions := test.NewSessionStore()
	var kept uuid.UUID
	keep := func(_ context.Context, id uuid.UUID) (bool, error) { return id == kept, nil }
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithSessionRetention(50*time.Millisecond, keep))
	require.NoError(t, err)
	require.NoError(t, r.Add("", newScripted()))
	create := func() uuid.UUID {
		session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
		require.NoError(t, err)
		return session.ID
	}
	old, kept, changed := create(), create(), create()
	time.Sleep(100 * time.Millisecond)

	// A session which changed since is kept, as is one the function keeps,
	// and the others, which haven't changed for longer, are deleted
	_, err = r.UpdateSession(ctx, changed, schema.SessionMeta{Title: types.Ptr("Still here")})
	require.NoError(t, err)
	fresh := create()
	assert.Equal(t, 1, r.DeleteOldSessions(ctx))
	_, err = r.GetSession(ctx, old)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	for _, id := range []uuid.UUID{kept, changed, fresh} {
		_, err = r.GetSession(ctx, id)
		assert.NoError(t, err)
	}

	// A session which was saved since it was listed, as old, isn't deleted,
	// as its serial isn't the one which was listed
	listed, err := r.GetSession(ctx, changed)
	require.NoError(t, err)
	_, err = r.UpdateSession(ctx, changed, schema.SessionMeta{Title: types.Ptr("Saved since")})
	require.NoError(t, err)
	assert.ErrorIs(t, r.DeleteSessionAt(ctx, changed, listed.Serial), llm.ErrConflict)
	_, err = r.GetSession(ctx, changed)
	assert.NoError(t, err)

	// Without retention, none is deleted
	r, err = registry.New(registry.WithSessionStore(sessions))
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, r.DeleteOldSessions(ctx))

	// Retention can't be negative
	_, err = registry.New(registry.WithSessionRetention(-time.Hour, nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
