package manager

import (
	"errors"
	"fmt"
	"testing"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

type sessionFeedMockRow struct {
	values []any
}

func (r sessionFeedMockRow) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan arity")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *uint64:
			*target = r.values[i].(uint64)
		case *uuid.UUID:
			*target = r.values[i].(uuid.UUID)
		case *string:
			*target = r.values[i].(string)
		case *[]schema.ContentBlock:
			*target = append((*target)[:0], r.values[i].([]schema.ContentBlock)...)
		case *uint:
			*target = r.values[i].(uint)
		case *map[string]any:
			*target = r.values[i].(map[string]any)
		case *time.Time:
			*target = r.values[i].(time.Time)
		default:
			return fmt.Errorf("unsupported scan type %T", dest[i])
		}
	}
	return nil
}

func Test_sessionFeedMessageListRequestSelect(t *testing.T) {
	assert := assert.New(t)
	limit := uint64(25)
	sessions := []uuid.UUID{uuid.New(), uuid.New()}
	b := pg.NewBind("schema", "llm", "message.session_feed", "SESSION_FEED")

	query, err := (sessionFeedMessageListRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit},
		Sessions:    sessions,
		AfterID:     42,
	}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("SESSION_FEED", query)
	assert.Equal(sessions, b.Get("sessions"))
	assert.Equal(uint64(42), b.Get("after_id"))
	assert.Equal("LIMIT 25", b.Get("offsetlimit"))
}

func Test_sessionFeedMessageListRequestSelectRequiresSessions(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")

	_, err := (sessionFeedMessageListRequest{}).Select(b, pg.List)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func Test_sessionFeedMessageScan(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(123, 0).UTC()
	session := uuid.New()
	text := "reply"
	meta := map[string]any{"source": "feed"}
	var message sessionFeedMessage

	err := message.Scan(sessionFeedMockRow{values: []any{
		uint64(55),
		session,
		schema.RoleAssistant,
		[]schema.ContentBlock{{Text: &text}},
		uint(12),
		schema.ResultToolCall.String(),
		meta,
		createdAt,
	}})
	if !assert.NoError(err) {
		return
	}

	assert.Equal(uint64(55), message.ID)
	assert.Equal(session, message.Session)
	assert.Equal(schema.RoleAssistant, message.Message.Role)
	assert.Equal(uint(12), message.Message.Tokens)
	assert.Equal(schema.ResultToolCall, message.Message.Result)
	assert.Equal(meta, message.Message.Meta)
	assert.Equal(createdAt, message.CreatedAt)
	if assert.Len(message.Message.Content, 1) {
		assert.NotNil(message.Message.Content[0].Text)
		assert.Equal(text, *message.Message.Content[0].Text)
	}
}
