package common_test

import (
	"net/http"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	common "github.com/mutablelogic/go-llm/webhook/protocol/common"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func contentType(value string) http.Header {
	h := http.Header{}
	if value != "" {
		h.Set("Content-Type", value)
	}
	return h
}

func Test_Common_Decode(t *testing.T) {
	// JSON, as it is, a form, as an object, text, as a string, and nothing,
	// or what isn't text, as nil
	values, err := common.Decode(contentType("application/json; charset=utf-8"), []byte(`{"a":1}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(values))
	values, err = common.Decode(contentType("application/vnd.api+json"), []byte(`[1]`))
	require.NoError(t, err)
	assert.JSONEq(t, `[1]`, string(values))
	_, err = common.Decode(contentType("application/json"), []byte(`{`))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	values, err = common.Decode(contentType("application/x-www-form-urlencoded"), []byte(`a=1&b=2&b=3`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":"1","b":["2","3"]}`, string(values))
	values, err = common.Decode(contentType("text/plain"), []byte("hello"))
	require.NoError(t, err)
	assert.JSONEq(t, `"hello"`, string(values))
	values, err = common.Decode(contentType(""), nil)
	require.NoError(t, err)
	assert.Nil(t, values)
	values, err = common.Decode(contentType("application/octet-stream"), []byte{0xff, 0xfe})
	require.NoError(t, err)
	assert.Nil(t, values)
}

func Test_Common_Field(t *testing.T) {
	values := []byte(`{"event":"x","event_id":100,"msg":{"id":"t1"},"n":1.50,"list":[1]}`)
	assert.Equal(t, "x", common.Field(values, "event"))
	assert.Equal(t, "100", common.Field(values, "event_id"))
	assert.Equal(t, "1.50", common.Field(values, "n"))
	assert.Equal(t, "t1", common.Field(values, "msg", "id"))
	assert.Empty(t, common.Field(values, "list"))
	assert.Empty(t, common.Field(values, "missing", "id"))
	assert.Empty(t, common.Field([]byte(`not json`), "event"))
}

func Test_Common_Equal(t *testing.T) {
	assert.True(t, common.Equal([]byte{1, 2}, []byte{1, 2}))
	assert.False(t, common.Equal([]byte{1, 2}, []byte{1, 3}))
	assert.False(t, common.Equal(nil, nil))
	assert.True(t, common.EqualString("a", "a"))
	assert.False(t, common.EqualString("", ""))
	assert.ErrorIs(t, common.NoSecret(), llm.ErrForbidden)
}
