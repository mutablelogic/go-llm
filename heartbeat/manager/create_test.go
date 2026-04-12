package heartbeat
package heartbeat_test

import (
	"testing"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/heartbeat/manager"
	uuid "github.com/google/uuid"
	kernel "github.com/mutablelogic/go-llm/kernel/schema"
	assert "github.com/stretchr/testify/assert"
)

func TestCreateRequestHeartbeatInsert(t *testing.T) {
	assert := assert.New(t)
	req := heartbeat.CreateRequest{
		Session:  uuid.New(),
		Message:  "daily reminder",
		Schedule: "0 9 * * 1-5",
		Timezone: "America/New_York",
	}

	insert, err := req.HeartbeatInsert()
	if !assert.NoError(err) {
		return
	}

	assert.Equal(req.Session, insert.Session)
	assert.Equal(req.Message, insert.Message)
	if assert.NotNil(insert.Schedule.Loc) {
		assert.Equal("America/New_York", insert.Schedule.Loc.String())
	}
}

func TestCreateRequestHeartbeatInsertRequiresSession(t *testing.T) {
	assert := assert.New(t)
	_, err := (heartbeat.CreateRequest{
		Message:  "daily reminder",
		Schedule: "0 9 * * 1-5",
	}).HeartbeatInsert()

	assert.Error(err)
	assert.ErrorIs(err, kernel.ErrBadParameter)
}