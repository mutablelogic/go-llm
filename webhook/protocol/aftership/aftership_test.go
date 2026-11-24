package aftership_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	aftership "github.com/mutablelogic/go-llm/webhook/protocol/aftership"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_AfterShip(t *testing.T) {
	p := aftership.New()
	secret, body := "webhook-secret", []byte(`{"event":"tracking_update","event_id":"e1","msg":{"id":"t1"},"ts":1700000000}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	signed := http.Header{}
	signed.Set("aftership-hmac-sha256", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	assert.NoError(t, p.Verify(signed, body, secret))
	assert.ErrorIs(t, p.Verify(signed, append(body, ' '), secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(http.Header{"Aftership-Hmac-Sha256": {"not base64!"}}, body, secret), llm.ErrForbidden)
	assert.Equal(t, []string{"Aftership-Hmac-Sha256"}, p.Headers())

	values, err := p.Decode(http.Header{"Content-Type": {"application/json"}}, body)
	require.NoError(t, err)
	event, delivery := p.Identify(http.Header{}, values)
	assert.Equal(t, "tracking_update", event)
	assert.Equal(t, "e1", delivery)
}
