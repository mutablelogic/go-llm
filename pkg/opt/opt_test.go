package opt_test

import (
	"testing"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	assert "github.com/stretchr/testify/assert"
)

func TestApplyEmpty(t *testing.T) {
	assert := assert.New(t)
	opts, err := opt.Apply()
	assert.NoError(err)
	assert.NotNil(opts)
	assert.False(opts.Has("missing"))
}

func TestStringOptions(t *testing.T) {
	assert := assert.New(t)
	opts, err := opt.Apply(opt.AddString("key", "value1", "value2"))
	assert.NoError(err)
	assert.Equal([]string{"value1", "value2"}, opts.GetStringArray("key"))
	assert.Equal("value1", opts.GetString("key"))
	query := opts.Query("key")
	assert.Equal([]string{"value1", "value2"}, query["key"])
}

func TestUintOptions(t *testing.T) {
	assert := assert.New(t)
	opts, err := opt.Apply(opt.AddUint("limit", 10, 20))
	assert.NoError(err)
	assert.Equal(uint(10), opts.GetUint("limit"))
	// Query should expose string slice
	assert.Equal([]string{"10", "20"}, opts.Query("limit")["limit"])
}

func TestFloatOptions(t *testing.T) {
	assert := assert.New(t)
	opts, err := opt.Apply(opt.AddFloat64("score", 1.5))
	assert.NoError(err)
	assert.InDelta(1.5, opts.GetFloat64("score"), 1e-9)
}

func TestBoolOptions(t *testing.T) {
	assert := assert.New(t)
	opts, err := opt.Apply(opt.SetBool("flag", true))
	assert.NoError(err)
	assert.True(opts.GetBool("flag"))
}

func TestToolkitStoredAsArbitrary(t *testing.T) {
	assert := assert.New(t)
	tk := struct{ Name string }{"toolkit"}
	opts, err := opt.Apply(opt.WithToolkit(tk))
	assert.NoError(err)
	assert.Equal(tk, opts.GetToolkit())
}

func TestQueryIgnoresNonStrings(t *testing.T) {
	assert := assert.New(t)
	opts, err := opt.Apply(opt.SetUint("number", 5))
	assert.NoError(err)
	// number stored as string via SetUint, so it should appear
	assert.Equal("5", opts.Query("number").Get("number"))
	// arbitrary non-string should not break Query
	opts.Set("obj", struct{}{})
	assert.Empty(opts.Query("obj").Get("obj"))
}
