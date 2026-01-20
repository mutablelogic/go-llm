package opt_test

import (
	"testing"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	assert "github.com/stretchr/testify/assert"
)

func Test_opt_001(t *testing.T) {
	assert := assert.New(t)

	// Apply with no options
	opts, err := opt.Apply()
	assert.NoError(err)
	assert.NotNil(opts)
}

func Test_opt_002(t *testing.T) {
	assert := assert.New(t)

	// Apply with WithString
	opts, err := opt.Apply(opt.WithString("key", "value"))
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("key")
	assert.Equal("value", query.Get("key"))
}

func Test_opt_003(t *testing.T) {
	assert := assert.New(t)

	// Apply with multiple string values
	opts, err := opt.Apply(opt.WithString("key", "value1", "value2"))
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("key")
	assert.Equal([]string{"value1", "value2"}, query["key"])
}

func Test_opt_004(t *testing.T) {
	assert := assert.New(t)

	// Apply with WithUint
	opts, err := opt.Apply(opt.WithUint("limit", 10))
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("limit")
	assert.Equal("10", query.Get("limit"))
}

func Test_opt_005(t *testing.T) {
	assert := assert.New(t)

	// Apply with multiple options
	opts, err := opt.Apply(
		opt.WithUint("limit", 25),
		opt.WithString("custom", "value"),
	)
	assert.NoError(err)
	assert.NotNil(opts)

	query := opts.Query("limit", "custom")
	assert.Equal("25", query.Get("limit"))
	assert.Equal("value", query.Get("custom"))
}

func Test_opt_006(t *testing.T) {
	assert := assert.New(t)

	// Query with non-existent key returns empty
	opts, err := opt.Apply(opt.WithString("key", "value"))
	assert.NoError(err)

	query := opts.Query("nonexistent")
	assert.Empty(query.Get("nonexistent"))
}
