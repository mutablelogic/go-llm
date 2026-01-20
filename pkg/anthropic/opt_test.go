package anthropic_test

import (
	"testing"

	// Packages
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	assert "github.com/stretchr/testify/assert"
)

func Test_opt_001(t *testing.T) {
	assert := assert.New(t)

	// Apply with WithAfterId
	opts, err := opt.Apply(anthropic.WithAfterId("abc123"))
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("after_id")
	assert.Equal("abc123", query.Get("after_id"))
}

func Test_opt_002(t *testing.T) {
	assert := assert.New(t)

	// Apply with WithBeforeId
	opts, err := opt.Apply(anthropic.WithBeforeId("xyz789"))
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("before_id")
	assert.Equal("xyz789", query.Get("before_id"))
}

func Test_opt_003(t *testing.T) {
	assert := assert.New(t)

	// Apply with WithLimit
	opts, err := opt.Apply(anthropic.WithLimit(50))
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("limit")
	assert.Equal("50", query.Get("limit"))
}

func Test_opt_004(t *testing.T) {
	assert := assert.New(t)

	// Apply with multiple anthropic options
	opts, err := opt.Apply(
		anthropic.WithAfterId("abc123"),
		anthropic.WithBeforeId("xyz789"),
		anthropic.WithLimit(25),
	)
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("after_id", "before_id", "limit")
	assert.Equal("abc123", query.Get("after_id"))
	assert.Equal("xyz789", query.Get("before_id"))
	assert.Equal("25", query.Get("limit"))
}
