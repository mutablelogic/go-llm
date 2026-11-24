package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Error is the Bot API's error, of a method: its code, which is an HTTP
// status, such as 429, its description, and what it says to do, which is
// how long to wait before calling it again, when it's rate limited, or the
// supergroup a group became, whose chat is used in its place
type Error struct {
	Method      string
	Code        int
	Description string
	Retry       time.Duration
	MigrateTo   int64
}

// responseParameters are what an error says to do
type responseParameters struct {
	RetryAfter      int   `json:"retry_after,omitempty"`
	MigrateToChatID int64 `json:"migrate_to_chat_id,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// maxAttempts is the most times a method which is rate limited is
	// called, and maxRetryWait the longest it waits to call it again, beyond
	// which its error is returned
	maxAttempts  = 3
	maxRetryWait = time.Minute

	// maxErrorBody is the most of an error's body which is read
	maxErrorBody = 64 << 10
)

var _ error = (*Error)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newError returns the error of the method's response, which isn't ok
func newError[T any](method string, r response[T]) *Error {
	e := &Error{Method: method, Code: r.ErrorCode, Description: r.Description}
	if r.Parameters != nil {
		e.Retry = time.Duration(r.Parameters.RetryAfter) * time.Second
		e.MigrateTo = r.Parameters.MigrateToChatID
	}
	return e
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Error returns the method, and its description, such as
// telegram sendMessage: Too Many Requests: retry after 5
func (e *Error) Error() string {
	return fmt.Sprintf("telegram %s: %s", e.Method, e.Description)
}

// RetryAfter returns how long to wait before calling the method again,
// when it's rate limited, or zero
func (e *Error) RetryAfter() time.Duration {
	return e.Retry
}

// Unwrap returns the kind of error its code is, such as ErrForbidden, for
// a token which isn't the bot's, and ErrServiceUnavailable, when it's
// rate limited
func (e *Error) Unwrap() error {
	switch e.Code {
	case http.StatusBadRequest:
		return llm.ErrBadParameter
	case http.StatusUnauthorized, http.StatusForbidden:
		return llm.ErrForbidden
	case http.StatusNotFound:
		return llm.ErrNotFound
	case http.StatusConflict:
		return llm.ErrConflict
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return llm.ErrServiceUnavailable
	}
	return llm.ErrInternalServerError
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// decodeError returns the function which reads the method's response, when
// it isn't a success, and returns its error, when it's the Bot API's, with
// what it says to do, or else leaves its body, as it was, to the client
func decodeError(method string) func(*http.Response) error {
	return func(resp *http.Response) error {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		if err != nil {
			return err
		}
		resp.Body = io.NopCloser(bytes.NewReader(data))
		var r response[json.RawMessage]
		if err := json.Unmarshal(data, &r); err != nil || r.OK || r.ErrorCode == 0 {
			return nil
		}
		return newError(method, r)
	}
}

// retry calls the function, and calls it again, when it's rate limited,
// after waiting as long as it says, up to maxAttempts times, unless it says
// to wait longer than maxRetryWait, or the context is cancelled first
func retry[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	for attempt := 1; ; attempt++ {
		result, err := fn()
		var e *Error
		if err == nil || !errors.As(err, &e) || e.Retry <= 0 || e.Retry > maxRetryWait || attempt >= maxAttempts {
			return result, err
		}
		trace.SpanFromContext(ctx).AddEvent("rate limited", trace.WithAttributes(
			attribute.String("method", e.Method),
			attribute.String("retry_after", e.Retry.String()),
			attribute.Int("attempt", attempt),
		))
		timer := time.NewTimer(e.Retry)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result, err
		case <-timer.C:
		}
	}
}
