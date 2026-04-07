package schema_test

import (
	"errors"
	"testing"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

type usageMockRow struct {
	values []any
}

func (r usageMockRow) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan arity")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *uint64:
			*target = r.values[i].(uint64)
		case *uint:
			*target = r.values[i].(uint)
		case *string:
			*target = r.values[i].(string)
		case *schema.UsageType:
			*target = r.values[i].(schema.UsageType)
		case **string:
			*target = r.values[i].(*string)
		case **uuid.UUID:
			switch value := r.values[i].(type) {
			case *uuid.UUID:
				*target = value
			case uuid.UUID:
				uuidValue := value
				*target = &uuidValue
			case nil:
				*target = nil
			default:
				return errors.New("unsupported uuid scan source")
			}
		case *schema.ProviderMetaMap:
			*target = r.values[i].(schema.ProviderMetaMap)
		case *time.Time:
			*target = r.values[i].(time.Time)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

func TestUsageScan(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(100, 0).UTC()
	sessionID := uuid.New()
	userID := uuid.New()
	meta := schema.ProviderMetaMap{"source": "test"}

	var usage schema.Usage
	err := usage.Scan(usageMockRow{values: []any{
		uint64(7),
		schema.UsageTypeAsk,
		func() *string { value := "batch-1"; return &value }(),
		&sessionID,
		userID,
		func() *string { value := "google-prod"; return &value }(),
		"gemini-2.5-pro",
		uint(18),
		uint(12),
		uint(5),
		uint(3),
		uint(2),
		meta,
		createdAt,
	}})
	if !assert.NoError(err) {
		return
	}

	assert.Equal(uint64(7), usage.ID)
	assert.Equal(schema.UsageTypeAsk, usage.UsageInsert.Type)
	if assert.NotNil(usage.UsageInsert.Batch) {
		assert.Equal("batch-1", *usage.UsageInsert.Batch)
	}
	assert.Equal(sessionID, usage.UsageInsert.Session)
	assert.Equal(userID, usage.UsageInsert.User)
	if assert.NotNil(usage.UsageInsert.Provider) {
		assert.Equal("google-prod", *usage.UsageInsert.Provider)
	}
	assert.Equal("gemini-2.5-pro", usage.UsageInsert.Model)
	assert.Equal(uint(18), usage.UsageInsert.InputTokens)
	assert.Equal(uint(12), usage.UsageInsert.OutputTokens)
	assert.Equal(uint(5), usage.UsageInsert.CacheReadTokens)
	assert.Equal(uint(3), usage.UsageInsert.CacheWriteTokens)
	assert.Equal(uint(2), usage.UsageInsert.ReasoningTokens)
	assert.Equal(meta, usage.UsageInsert.Meta)
	assert.Equal(createdAt, usage.CreatedAt)
}

func TestUsageScanNullUUIDsBecomeNilValues(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(101, 0).UTC()
	meta := schema.ProviderMetaMap{"source": "test"}

	var usage schema.Usage
	err := usage.Scan(usageMockRow{values: []any{
		uint64(8),
		schema.UsageTypeAsk,
		(*string)(nil),
		(*uuid.UUID)(nil),
		(*uuid.UUID)(nil),
		(*string)(nil),
		"phi4:latest",
		uint(18),
		uint(12),
		uint(0),
		uint(0),
		uint(0),
		meta,
		createdAt,
	}})
	if !assert.NoError(err) {
		return
	}

	assert.Equal(uuid.Nil, usage.User)
	assert.Equal(uuid.Nil, usage.Session)
	assert.Nil(usage.Batch)
	assert.Nil(usage.Provider)
	assert.Equal("phi4:latest", usage.Model)
	assert.Equal(meta, usage.Meta)
	assert.Equal(createdAt, usage.CreatedAt)
}

func TestUsageInsert(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "usage.insert", "INSERT")
	sessionID := uuid.New()
	userID := uuid.New()
	batch := "batch-9"
	provider := "ollama-primary"

	query, err := (schema.UsageInsert{
		Type:     schema.UsageTypeChat,
		Batch:    &batch,
		Session:  sessionID,
		User:     userID,
		Provider: &provider,
		Model:    "llama3.2",
		UsageMeta: schema.UsageMeta{
			InputTokens:      100,
			OutputTokens:     25,
			CacheReadTokens:  7,
			CacheWriteTokens: 2,
			ReasoningTokens:  4,
			Meta:             schema.ProviderMetaMap{"tenant": "acme"},
		},
	}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("INSERT", query)
	assert.Equal(schema.UsageTypeChat, b.Get("type"))
	assert.Equal("batch-9", b.Get("batch"))
	assert.Equal(sessionID, b.Get("session"))
	assert.Equal(userID, b.Get("user"))
	assert.Equal("ollama-primary", b.Get("provider"))
	assert.Equal("llama3.2", b.Get("model"))
	assert.Equal(uint(100), b.Get("input_tokens"))
	assert.Equal(uint(25), b.Get("output_tokens"))
	assert.Equal(uint(7), b.Get("cache_read_tokens"))
	assert.Equal(uint(2), b.Get("cache_write_tokens"))
	assert.Equal(uint(4), b.Get("reasoning_tokens"))
	assert.Equal(schema.ProviderMetaMap{"tenant": "acme"}, b.Get("meta"))
}

func TestUsageInsertNilUUIDsBindNull(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "usage.insert", "INSERT")

	_, err := (schema.UsageInsert{
		Type:  schema.UsageTypeAsk,
		Model: "gemini-2.5-pro",
	}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Nil(b.Get("user"))
	assert.Nil(b.Get("session"))
}

func TestUsageInsertRequiresTypeAndModel(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "usage.insert", "INSERT")

	_, err := (schema.UsageInsert{Model: "gemini-2.5-pro"}).Insert(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}

	_, err = (schema.UsageInsert{Type: schema.UsageTypeAsk}).Insert(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}
