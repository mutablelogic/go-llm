package schema_test

import (
	"errors"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	assert "github.com/stretchr/testify/assert"
)

func TestHTTPErrPassThrough(t *testing.T) {
	assert := assert.New(t)
	input := httpresponse.ErrBadRequest.With("bad input")
	output := schema.HTTPErr(input)

	var code httpresponse.Err
	if assert.Error(output) && assert.True(errors.As(output, &code)) {
		assert.Equal(httpresponse.ErrBadRequest, code)
	}
	assert.Equal(input.Error(), output.Error())
}

func TestHTTPErrSchemaMapping(t *testing.T) {
	assert := assert.New(t)
	output := schema.HTTPErr(schema.ErrNotFound.With("provider missing"))

	var code httpresponse.Err
	if assert.Error(output) && assert.True(errors.As(output, &code)) {
		assert.Equal(httpresponse.ErrNotFound, code)
	}
	assert.ErrorContains(output, "provider missing")
}

func TestHTTPErrFallback(t *testing.T) {
	assert := assert.New(t)
	output := schema.HTTPErr(errors.New("boom"))

	var code httpresponse.Err
	if assert.Error(output) && assert.True(errors.As(output, &code)) {
		assert.Equal(httpresponse.ErrInternalError, code)
	}
	assert.ErrorContains(output, "boom")
}

func TestHTTPErrPGMapping(t *testing.T) {
	assert := assert.New(t)
	output := schema.HTTPErr(pg.ErrConflict.With("duplicate key value violates unique constraint \"provider_pkey\""))

	var code httpresponse.Err
	if assert.Error(output) && assert.True(errors.As(output, &code)) {
		assert.Equal(httpresponse.ErrConflict, code)
	}
	assert.ErrorContains(output, "duplicate key value")
}

func TestHTTPErrNil(t *testing.T) {
	assert := assert.New(t)
	assert.NoError(schema.HTTPErr(nil))
}
