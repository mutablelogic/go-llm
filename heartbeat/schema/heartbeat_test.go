package schema_test

import (
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	kernel "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

func TestHeartbeatInsert(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "heartbeat", "llm", "llm", "heartbeat.insert", "INSERT")
	schedule, err := schema.NewTimeSpec("0 9 * * 1-5", nil)
	if !assert.NoError(err) {
		return
	}

	session := uuid.New()
	query, err := (schema.HeartbeatInsert{
		Session: session,
		HeartbeatMeta: schema.HeartbeatMeta{
			Message:  "daily reminder",
			Schedule: schedule,
		},
	}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("INSERT", query)
	assert.Equal(session, b.Get("session"))
	assert.Equal("daily reminder", b.Get("message"))
	assert.Equal(schedule, b.Get("schedule"))
}

func TestHeartbeatInsertRequiresSession(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "heartbeat", "llm", "llm", "heartbeat.insert", "INSERT")
	schedule, err := schema.NewTimeSpec("0 9 * * 1-5", nil)
	if !assert.NoError(err) {
		return
	}

	_, err = (schema.HeartbeatInsert{
		HeartbeatMeta: schema.HeartbeatMeta{
			Message:  "daily reminder",
			Schedule: schedule,
		},
	}).Insert(b)
	assert.Error(err)
	assert.ErrorIs(err, kernel.ErrBadParameter)
}
