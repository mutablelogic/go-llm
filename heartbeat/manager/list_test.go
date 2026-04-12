package heartbeat_test

import (
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

func TestHeartbeatListRequestSelect(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("heartbeat.list", "LIST_ALL", "heartbeat.list_for_user", "LIST_USER")
	fired := false

	query, err := (schema.HeartbeatListRequest{Fired: &fired}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST_ALL", query)
	assert.Equal("WHERE fired=@fired", b.Get("where"))
	assert.Equal(false, b.Get("fired"))
	assert.Equal("LIMIT 100", b.Get("offsetlimit"))
}

func TestHeartbeatListRequestSelectForUser(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("heartbeat.list", "LIST_ALL", "heartbeat.list_for_user", "LIST_USER")
	b.Set("user", uuid.New())
	fired := false

	query, err := (schema.HeartbeatListRequest{Fired: &fired}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST_USER", query)
	assert.Equal("AND fired=@fired", b.Get("where"))
	assert.Equal(false, b.Get("fired"))
	assert.Equal("LIMIT 100", b.Get("offsetlimit"))
}

func TestHeartbeatListRequestSelectForUserNoFilters(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("heartbeat.list", "LIST_ALL", "heartbeat.list_for_user", "LIST_USER")
	b.Set("user", uuid.New())

	query, err := (schema.HeartbeatListRequest{}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST_USER", query)
	assert.Equal("", b.Get("where"))
	assert.Equal("LIMIT 100", b.Get("offsetlimit"))
}
