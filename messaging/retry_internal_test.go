package messaging

import (
	"errors"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

// rateLimited is an error which says how long to wait
type rateLimited time.Duration

func (e rateLimited) Error() string             { return "rate limited" }
func (e rateLimited) RetryAfter() time.Duration { return time.Duration(e) }

func Test_Messaging_RetryDelay(t *testing.T) {
	// The wait, when the error doesn't say, or says less
	delay, ok := retryDelay(5*time.Second, nil)
	assert.True(t, ok)
	assert.Equal(t, 5*time.Second, delay)
	delay, ok = retryDelay(5*time.Second, errors.New("network"))
	assert.True(t, ok)
	assert.Equal(t, 5*time.Second, delay)
	delay, ok = retryDelay(5*time.Second, rateLimited(time.Second))
	assert.True(t, ok)
	assert.Equal(t, 5*time.Second, delay)

	// As long as the error says, when it's longer, wrapped or not
	delay, ok = retryDelay(5*time.Second, rateLimited(32*time.Second))
	assert.True(t, ok)
	assert.Equal(t, 32*time.Second, delay)
	delay, ok = retryDelay(5*time.Second, errors.Join(errors.New("poll"), rateLimited(time.Minute)))
	assert.True(t, ok)
	assert.Equal(t, time.Minute, delay)
	assert.Equal(t, time.Minute, RetryAfter(errors.Join(errors.New("poll"), rateLimited(time.Minute))))
	assert.Zero(t, RetryAfter(errors.New("network")))

	// Not again, when it's forbidden
	_, ok = retryDelay(5*time.Second, llm.ErrForbidden.With("Unauthorized"))
	assert.False(t, ok)
}
