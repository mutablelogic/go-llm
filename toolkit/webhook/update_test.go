package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Update(t *testing.T) {
	s := newServer(t)
	tk, session := newToolkit(t, s), uuid.New()
	var log strings.Builder
	ctx := logSession(t.Context(), session, func(_ llm.StreamType, text string) { log.WriteString(text) })
	call := func(ctx context.Context, name, input string, result any) (string, error) {
		data, err := tk.CallTool(ctx, "webhook:"+name, json.RawMessage(input))
		if err == nil {
			require.NoError(t, json.Unmarshal(data, result))
		}
		return string(data), err
	}
	create := func(session uuid.UUID) schema.WebhookToken {
		w, err := s.registry.CreateWebhook(t.Context(), schema.WebhookMeta{Session: session, Description: "Pidgeon", Type: "pidgeon"})
		require.NoError(t, err)
		return w
	}
	a, other := create(session), create(uuid.New())

	// Open, until it's given the secret, which its service gave once its URL
	// was registered
	var tested testOutput
	_, err := call(ctx, "test", fmt.Sprintf(`{"webhook_id":%d}`, a.ID), &tested)
	require.NoError(t, err)
	assert.Equal(t, "not_checked", tested.Signature)
	var updated updateOutput
	text, err := call(ctx, "update", fmt.Sprintf(`{"webhook_id":%d,"secret":" whsec_1 "}`, a.ID), &updated)
	require.NoError(t, err)
	assert.True(t, updated.HasSecret)
	assert.Equal(t, "Pidgeon", updated.Description)
	assert.Contains(t, updated.Note, "signed with its secret")
	assert.NotContains(t, text, "whsec_1")
	assert.NotContains(t, log.String(), "whsec_1")

	// Its calls are verified, at the same URL, with the secret as it was given
	for signature, expected := range map[string]string{"good": "verified", "bad": "failed", "none": "failed"} {
		_, err = call(ctx, "test", fmt.Sprintf(`{"webhook_id":%d,"signature":%q}`, a.ID, signature), &tested)
		require.NoError(t, err)
		assert.Equal(t, expected, tested.Signature, signature)
	}
	_, err = s.registry.Call(t.Context(), a.Token, newRequest("{}"))
	assert.ErrorIs(t, err, llm.ErrForbidden)
	stored, err := s.store.GetWebhook(t.Context(), a.ID)
	require.NoError(t, err)
	assert.Equal(t, "whsec_1", stored.Secret)

	// Its description, which keeps its secret, as an empty one isn't one
	_, err = call(ctx, "update", fmt.Sprintf(`{"webhook_id":%d,"description":" Mail ","secret":""}`, a.ID), &updated)
	require.NoError(t, err)
	assert.Equal(t, "Mail", updated.Description)
	stored, err = s.store.GetWebhook(t.Context(), a.ID)
	require.NoError(t, err)
	assert.Equal(t, "whsec_1", stored.Secret)

	// Its secret is replaced
	_, err = call(ctx, "update", fmt.Sprintf(`{"webhook_id":%d,"secret":"whsec_2"}`, a.ID), &updated)
	require.NoError(t, err)
	assert.Equal(t, "Mail", updated.Description)
	stored, err = s.store.GetWebhook(t.Context(), a.ID)
	require.NoError(t, err)
	assert.Equal(t, "whsec_2", stored.Secret)

	// Not without a change, or a webhook, or a session, or another session's
	for _, input := range []string{``, `{"secret":"x"}`, fmt.Sprintf(`{"webhook_id":%d}`, a.ID), fmt.Sprintf(`{"webhook_id":%d,"secret":" ","description":" "}`, a.ID)} {
		_, err = call(ctx, "update", input, &updated)
		assert.Error(t, err, input)
	}
	_, err = call(t.Context(), "update", fmt.Sprintf(`{"webhook_id":%d,"secret":"x"}`, a.ID), &updated)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	for _, id := range []uint64{other.ID, 999} {
		_, err = call(ctx, "update", fmt.Sprintf(`{"webhook_id":%d,"secret":"x"}`, id), &updated)
		assert.ErrorIs(t, err, llm.ErrNotFound)
	}
	stored, err = s.store.GetWebhook(t.Context(), other.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.Secret)
}
