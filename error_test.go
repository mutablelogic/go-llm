package llm_test

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
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

func Test_HTTPStatus(t *testing.T) {
	// The status of an error which wraps one, or of a server's response,
	// which a client returns for a JSON body
	for err, want := range map[error]int{
		httpresponse.ErrNotFound.With("missing"):                                     http.StatusNotFound,
		fmt.Errorf("wrapped: %w", httpresponse.ErrBadRequest):                        http.StatusBadRequest,
		httpresponse.ErrResponse{Code: http.StatusNotFound, Reason: "Not Found"}:     http.StatusNotFound,
		fmt.Errorf("wrapped: %w", httpresponse.ErrResponse{Code: http.StatusTeapot}): http.StatusTeapot,
	} {
		status, ok := llm.HTTPStatus(err)
		assert.True(t, ok, err)
		assert.Equal(t, want, status, err)
	}

	// But not another error
	for _, err := range []error{errors.New("other"), httpresponse.ErrResponse{}, nil} {
		_, ok := llm.HTTPStatus(err)
		assert.False(t, ok, err)
	}

	// And a server's response is its status, as a HTTP error
	var code httpresponse.Err
	if assert.True(t, errors.As(llm.HTTPErr(httpresponse.ErrResponse{Code: http.StatusNotFound}), &code)) {
		assert.Equal(t, httpresponse.ErrNotFound, code)
	}
}

func Test_HTTPErrMessage(t *testing.T) {
	// An llm.Err's text, which the status says, isn't repeated, but is kept
	// when it says more, or doesn't start the message
	for err, want := range map[error]string{
		llm.ErrNotFound.Withf("model %q", "x"):    `Not Found: model "x"`,
		llm.ErrBadParameter.With("missing model"): "Bad Request: missing model",
		llm.ErrNotFound: "Not Found",
		llm.ErrServiceUnavailable.Withf("%w", errors.New("down")): "Service Unavailable: down",
		llm.ErrMaxTokens.With("after 100"):                        "Bad Request: response truncated: max tokens reached: after 100",
		fmt.Errorf("provider: %w", llm.ErrNotFound):               "Not Found: provider: not found",
	} {
		result := llm.HTTPErr(err)
		assert.Equal(t, want, result.Error(), err)

		// And it still matches the status, and the llm.Err
		var code httpresponse.Err
		assert.True(t, errors.As(result, &code), err)
		var e llm.Err
		assert.True(t, errors.As(result, &e), err)
	}

	// And the status is the response's code, and the message is its reason
	w := httptest.NewRecorder()
	require.NoError(t, httpresponse.Error(w, llm.HTTPErr(llm.ErrNotFound.With("model"))))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"reason": "Not Found: model"`)
}
