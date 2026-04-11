package manager

import (
	"context"
	"errors"
	"testing"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
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
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

func TestNewSessionFeed(t *testing.T) {
	assert := assert.New(t)
	_, m := newIntegrationManager(t)
	feed, err := NewSessionFeed(context.Background(), m.PoolConn, time.Second)
	if !assert.NoError(err) {
		return
	}
	if assert.NotNil(feed) {
		assert.NotNil(feed.Conn)
		assert.Equal(time.Second, feed.delay)
	}
}

func TestSessionFeedUpdateDelay(t *testing.T) {
	assert := assert.New(t)
	_, m := newIntegrationManager(t)
	feed, err := NewSessionFeed(context.Background(), m.PoolConn, time.Hour)
	if !assert.NoError(err) {
		return
	}
	if assert.NotNil(feed) {
		feed.next = time.Now().Add(time.Hour)
	}
	assert.NoError(feed.update(context.Background()))
}

func TestSessionFeedSubscribeUnsubscribe(t *testing.T) {
	assert := assert.New(t)
	_, m := newIntegrationManager(t)
	feed, err := NewSessionFeed(context.Background(), m.PoolConn, time.Second)
	if !assert.NoError(err) {
		return
	}

	session := uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = feed.Subscribe(ctx, session, func([]*schema.Message) {})
	if !assert.NoError(err) {
		return
	}

	if assert.Contains(feed.subscribers, session) {
		assert.Len(feed.subscribers[session], 1)
	}
	cancel()
	llmtest.WaitUntil(t, time.Second, func() bool {
		feed.mu.Lock()
		defer feed.mu.Unlock()
		_, exists := feed.subscribers[session]
		return !exists
	}, "timed out waiting for session subscription cleanup")
}

func TestManagerSubscribeSession(t *testing.T) {
	assert := assert.New(t)
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	owner := llmtest.AdminUser(conn)
	other := llmtest.User(conn)
	modelName := llmtest.ModelNameMatching(t, "", syncAndListModels(m, provider.Name, owner), func(model schema.Model) bool {
		return model.Cap&schema.ModelCapCompletion != 0
	}, validateAccessibleModel(m, provider.Name, owner))

	session, err := m.CreateSession(ctx, schema.SessionInsert{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(modelName), Provider: types.Ptr(provider.Name)},
		},
	}, owner)
	if !assert.NoError(err) {
		return
	}

	subscribeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	err = m.SubscribeSession(subscribeCtx, session.ID, func([]*schema.Message) {}, owner)
	if !assert.NoError(err) {
		return
	}

	err = m.SubscribeSession(subscribeCtx, session.ID, func([]*schema.Message) {}, other)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrNotFound)
	}
}

func TestSessionFeedUpdateDispatchesMessages(t *testing.T) {
	assert := assert.New(t)
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	admin := llmtest.AdminUser(conn)
	modelName := llmtest.ModelNameMatching(t, "", syncAndListModels(m, provider.Name, admin), func(model schema.Model) bool {
		return model.Cap&schema.ModelCapCompletion != 0
	}, validateAccessibleModel(m, provider.Name, admin))

	session, err := m.CreateSession(ctx, schema.SessionInsert{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(modelName), Provider: types.Ptr(provider.Name)},
			Title:         types.Ptr("feed"),
		},
	}, admin)
	if !assert.NoError(err) {
		return
	}

	feed, err := NewSessionFeed(ctx, m.PoolConn, 0)
	if !assert.NoError(err) {
		return
	}

	var calls int
	var delivered []*schema.Message
	subscribeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	err = feed.Subscribe(subscribeCtx, session.ID, func(messages []*schema.Message) {
		calls++
		delivered = append(delivered, messages...)
	})
	if !assert.NoError(err) {
		return
	}

	if err := m.PoolConn.Insert(ctx, nil, schema.MessageInsert{
		Session: session.ID,
		Message: schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: types.Ptr("hello")}}, Tokens: 1},
	}); !assert.NoError(err) {
		return
	}
	if err := m.PoolConn.Insert(ctx, nil, schema.MessageInsert{
		Session: session.ID,
		Message: schema.Message{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: types.Ptr("world")}}, Tokens: 1, Result: schema.ResultStop},
	}); !assert.NoError(err) {
		return
	}

	if !assert.NoError(feed.update(ctx)) {
		return
	}

	assert.Equal(1, calls)
	if assert.Len(delivered, 2) {
		assert.Equal(schema.RoleUser, delivered[0].Role)
		assert.Equal(schema.RoleAssistant, delivered[1].Role)
	}
}

func TestMessageLastIDSelectorSelect(t *testing.T) {
	assert := assert.New(t)
	sessions := []uuid.UUID{uuid.New(), uuid.New()}
	b := pg.NewBind("schema", "llm", "message.last_id", "LAST_ID")

	query, err := (messageLastIDSelector{Sessions: sessions}).Select(b, pg.Get)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LAST_ID", query)
	assert.Equal(sessions, b.Get("sessions"))
	assert.Contains(b.Get("where").(string), `message.session = ANY(`)
}

func TestMessageLastIDSelectorSelectWithoutSessions(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "message.last_id", "LAST_ID")

	query, err := (messageLastIDSelector{}).Select(b, pg.Get)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LAST_ID", query)
	assert.Equal("", b.Get("where"))
}

func TestMessageLastIDScan(t *testing.T) {
	assert := assert.New(t)
	var result messageLastID
	err := result.Scan(sessionFeedMockRow{values: []any{uint64(12)}})
	if !assert.NoError(err) {
		return
	}
	assert.Equal(uint64(12), uint64(result))
}
