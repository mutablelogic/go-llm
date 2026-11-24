package pidgeon

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

func Test_Pidgeon(t *testing.T) {
	at := time.Unix(1794683000, 0)
	now = func() time.Time { return at }
	defer func() { now = time.Now }()

	p := New()
	secret, body := "whsec_test", []byte(`{"id":"evt_1","type":"message.received","created":"2026-11-24T09:00:00Z","data":{"inbox_id":"in_1"}}`)
	signature := func(seconds int64, secret string) string {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(strconv.FormatInt(seconds, 10) + "."))
		mac.Write(body)
		return hex.EncodeToString(mac.Sum(nil))
	}
	signed := func(value string) http.Header { return http.Header{"Pidgeon-Signature": {value}} }
	header := func(seconds int64, signatures ...string) http.Header {
		return signed("t=" + strconv.FormatInt(seconds, 10) + ",v1=" + strings.Join(signatures, ",v1="))
	}

	// Signed, now, or within five minutes, with one of its signatures
	assert.NoError(t, p.Verify(header(at.Unix(), signature(at.Unix(), secret)), body, secret))
	assert.NoError(t, p.Verify(header(at.Unix()-299, signature(at.Unix()-299, secret)), body, secret))
	assert.NoError(t, p.Verify(header(at.Unix(), signature(at.Unix(), "old"), signature(at.Unix(), secret)), body, secret))

	// Not, when it's too old, or the body, or secret, isn't, or it's missing
	assert.ErrorIs(t, p.Verify(header(at.Unix()-301, signature(at.Unix()-301, secret)), body, secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(header(at.Unix()+301, signature(at.Unix()+301, secret)), body, secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(header(at.Unix(), signature(at.Unix(), secret)), append(body, ' '), secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(header(at.Unix(), signature(at.Unix(), "other")), body, secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(signed("v1=00"), body, secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(signed(""), body, secret), llm.ErrForbidden)
	assert.ErrorIs(t, p.Verify(header(at.Unix(), signature(at.Unix(), secret)), body, ""), llm.ErrForbidden)

	// The event, from its header, or the body, and the delivery, from the body
	event, delivery := p.Identify(http.Header{"Pidgeon-Event": {"message.received"}}, body)
	assert.Equal(t, "message.received", event)
	assert.Equal(t, "evt_1", delivery)
	event, _ = p.Identify(http.Header{}, body)
	assert.Equal(t, "message.received", event)
}
