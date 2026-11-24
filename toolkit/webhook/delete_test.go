package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Delete(t *testing.T) {
	s := newServer(t)
	tk, session := newToolkit(t, s), uuid.New()
	ctx := inSession(t.Context(), session)
	remove := func(ctx context.Context, input string) (result removeOutput, err error) {
		data, err := tk.CallTool(ctx, "webhook:delete", json.RawMessage(input))
		if err == nil {
			require.NoError(t, json.Unmarshal(data, &result))
		}
		return result, err
	}
	a, b, other := newWebhook(t, s, session), newWebhook(t, s, session), newWebhook(t, s, uuid.New())
	for _, w := range []schema.WebhookToken{a, a, b, other} {
		s.call(t, w, "called")
	}
	calls := func() (result []uint64) {
		list, err := s.registry.ListCalls(t.Context(), schema.CallListRequest{})
		require.NoError(t, err)
		for _, c := range list.Body {
			result = append(result, c.ID)
		}
		return
	}

	// Not both, or neither, or another session's, or without a session
	for _, input := range []string{``, `{}`, `{"webhook_id":0}`, fmt.Sprintf(`{"webhook_id":%d,"call_id":1}`, a.ID)} {
		_, err := remove(ctx, input)
		assert.Error(t, err, input)
	}
	for _, input := range []string{fmt.Sprintf(`{"webhook_id":%d}`, other.ID), `{"call_id":4}`, `{"webhook_id":999}`, `{"call_id":999}`} {
		_, err := remove(ctx, input)
		assert.ErrorIs(t, err, llm.ErrNotFound, input)
	}
	for _, input := range []string{fmt.Sprintf(`{"webhook_id":%d}`, a.ID), `{"call_id":1}`} {
		_, err := remove(t.Context(), input)
		assert.ErrorIs(t, err, llm.ErrBadParameter, input)
	}
	require.Equal(t, []uint64{1, 2, 3, 4}, calls())

	// A call, and not the others of its webhook
	result, err := remove(ctx, `{"call_id":1}`)
	require.NoError(t, err)
	assert.Nil(t, result.Webhook)
	assert.Equal(t, uint64(1), result.Call.ID)
	assert.Equal(t, []uint64{2, 3, 4}, calls())
	_, err = remove(ctx, `{"call_id":1}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// A webhook, with its calls, whose URL no longer works
	result, err = remove(ctx, fmt.Sprintf(`{"webhook_id":%d}`, a.ID))
	require.NoError(t, err)
	assert.Nil(t, result.Call)
	assert.Equal(t, newWebhookOutput(a.Webhook), *result.Webhook)
	assert.Equal(t, []uint64{3, 4}, calls())
	_, err = s.registry.Call(t.Context(), a.Token, newRequest("x"))
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = s.registry.GetWebhook(t.Context(), b.ID)
	assert.NoError(t, err)
	_, err = remove(ctx, fmt.Sprintf(`{"webhook_id":%d}`, a.ID))
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// Not when the server can't be reached
	s.down.Store(true)
	_, err = remove(ctx, fmt.Sprintf(`{"webhook_id":%d}`, b.ID))
	assert.Error(t, err)
}
