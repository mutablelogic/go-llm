package llm_test

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	assert "github.com/stretchr/testify/assert"
)

func Test_Withf(t *testing.T) {
	// The error wraps the code, and the errors the format wraps with %w
	cause := errors.New("cause")
	err := llm.ErrBadParameter.Withf("input: %w", cause)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.ErrorIs(t, err, cause)
	assert.Equal(t, "bad parameter: input: cause", err.Error())
}

func Test_HTTPErr(t *testing.T) {
	t.Run("Err", func(t *testing.T) {
		for err, want := range map[llm.Err]httpresponse.Err{
			llm.ErrNotFound:            httpresponse.ErrNotFound,
			llm.ErrBadParameter:        httpresponse.ErrBadRequest,
			llm.ErrConflict:            httpresponse.ErrConflict,
			llm.ErrNotImplemented:      httpresponse.ErrNotImplemented,
			llm.ErrInternalServerError: httpresponse.ErrInternalError,
			llm.ErrServiceUnavailable:  httpresponse.ErrServiceUnavailable,
			llm.ErrForbidden:           httpresponse.ErrForbidden,
			llm.ErrMaxTokens:           httpresponse.ErrBadRequest,
			llm.ErrRefusal:             httpresponse.ErrBadRequest,
			llm.ErrPauseTurn:           httpresponse.ErrInternalError,
		} {
			var code httpresponse.Err
			if assert.True(t, errors.As(llm.HTTPErr(err.With("test")), &code), err) {
				assert.Equal(t, want, code, err.Error())
			}
		}
	})

	t.Run("Other", func(t *testing.T) {
		for err, want := range map[error]httpresponse.Err{
			fs.ErrNotExist:        httpresponse.ErrNotFound,
			fs.ErrExist:           httpresponse.ErrConflict,
			fs.ErrPermission:      httpresponse.ErrForbidden,
			io.ErrUnexpectedEOF:   httpresponse.ErrBadRequest,
			errors.New("unknown"): httpresponse.ErrInternalError,
		} {
			var code httpresponse.Err
			wrapped := fmt.Errorf("wrapped: %w", err)
			if assert.True(t, errors.As(llm.HTTPErr(wrapped), &code), err) {
				assert.Equal(t, want, code, err.Error())
				assert.ErrorIs(t, llm.HTTPErr(wrapped), err)
			}
		}
	})

	t.Run("Wrapped", func(t *testing.T) {
		// An llm error which wraps a HTTP status, such as a provider's
		// response, is the llm error's status
		err := llm.ErrServiceUnavailable.Withf("%w", httpresponse.ErrInternalError.With("provider failed"))
		var code httpresponse.Err
		if assert.True(t, errors.As(llm.HTTPErr(err), &code)) {
			assert.Equal(t, httpresponse.ErrServiceUnavailable, code)
		}
		assert.ErrorIs(t, llm.HTTPErr(err), httpresponse.ErrInternalError)
	})

	t.Run("Unchanged", func(t *testing.T) {
		assert.Nil(t, llm.HTTPErr(nil))
		err := httpresponse.ErrNotFound.With("test")
		assert.Equal(t, err, llm.HTTPErr(err))
	})
}
