package httpclient

import (
	"encoding/json"
	"testing"
	"time"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

func Test_Cache(t *testing.T) {
	// The oldest is replaced, once there are as many as are kept, and one
	// isn't returned once it's too old
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	c := newCache(2, time.Hour)
	c.now = func() time.Time { return now }
	has := func(key string) bool {
		_, exists := c.get(key)
		return exists
	}
	c.set("a", json.RawMessage(`1`))
	now = now.Add(time.Minute)
	c.set("b", json.RawMessage(`2`))
	now = now.Add(time.Minute)
	c.set("b", json.RawMessage(`3`))
	assert.True(t, has("a"))
	c.set("c", json.RawMessage(`4`))
	assert.False(t, has("a"))
	assert.True(t, has("b"))
	assert.True(t, has("c"))
	data, _ := c.get("b")
	assert.JSONEq(t, `3`, string(data))
	now = now.Add(time.Hour)
	assert.False(t, has("b"))
	assert.Empty(t, c.entries["b"].data)
}
