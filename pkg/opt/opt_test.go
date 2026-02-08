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
	opts, err := opt.Apply(opt.SetAny(opt.ToolkitKey, tk))
	assert.NoError(err)
	assert.Equal(tk, opts.Get(opt.ToolkitKey))
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

func TestSetAny(t *testing.T) {
	assert := assert.New(t)

	// Store an arbitrary struct
	type item struct{ Name string }
	opts, err := opt.Apply(opt.SetAny("item", item{"first"}))
	assert.NoError(err)
	assert.Equal(item{"first"}, opts.Get("item"))

	// SetAny replaces existing value
	opts, err = opt.Apply(
		opt.SetAny("item", item{"first"}),
		opt.SetAny("item", item{"second"}),
	)
	assert.NoError(err)
	assert.Equal(item{"second"}, opts.Get("item"))
}

func TestAddAny(t *testing.T) {
	assert := assert.New(t)

	// Single value creates a slice
	type block struct{ ID int }
	opts, err := opt.Apply(opt.AddAny("blocks", block{1}))
	assert.NoError(err)
	result, ok := opts.Get("blocks").([]block)
	assert.True(ok)
	assert.Equal([]block{{1}}, result)
}

func TestAddAnyMultiple(t *testing.T) {
	assert := assert.New(t)

	// Multiple values accumulate
	type block struct{ ID int }
	opts, err := opt.Apply(
		opt.AddAny("blocks", block{1}),
		opt.AddAny("blocks", block{2}),
		opt.AddAny("blocks", block{3}),
	)
	assert.NoError(err)
	result, ok := opts.Get("blocks").([]block)
	assert.True(ok)
	assert.Equal([]block{{1}, {2}, {3}}, result)
}

func TestAddAnyTypeMismatch(t *testing.T) {
	assert := assert.New(t)

	// Mismatched types should error
	_, err := opt.Apply(
		opt.AddAny("key", "string_value"),
		opt.AddAny("key", 42),
	)
	assert.Error(err)
}

func TestAddAnyWithStrings(t *testing.T) {
	assert := assert.New(t)

	// Works with primitive types too
	opts, err := opt.Apply(
		opt.AddAny("tags", "alpha"),
		opt.AddAny("tags", "beta"),
	)
	assert.NoError(err)
	result, ok := opts.Get("tags").([]string)
	assert.True(ok)
	assert.Equal([]string{"alpha", "beta"}, result)
}
