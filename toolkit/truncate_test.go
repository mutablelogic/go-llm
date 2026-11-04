package toolkit_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// bytesPerToken returns a function which measures text as a token for every
// bytes, or part of them, and counts how many times it's called
func bytesPerToken(bytes int, calls *int) toolkit.MeasureFn {
	return func(_ context.Context, text string) (uint, error) {
		if calls != nil {
			*calls++
		}
		return uint((len(text) + bytes - 1) / bytes), nil
	}
}

// truncate truncates the result to the limit, measured as a token for every
// four bytes, and asserts it's valid JSON which fits
func truncate(t *testing.T, result string, limit uint) json.RawMessage {
	t.Helper()
	measure := bytesPerToken(4, nil)
	truncated, err := toolkit.TruncateResult(context.Background(), json.RawMessage(result), limit, measure)
	require.NoError(t, err)
	require.True(t, json.Valid(truncated), string(truncated))
	require.True(t, utf8.Valid(truncated))
	tokens, _ := measure(context.Background(), string(truncated))
	assert.LessOrEqual(t, tokens, limit, string(truncated))
	return truncated
}

func Test_TruncateResult(t *testing.T) {
	ctx := context.Background()

	t.Run("Fits", func(t *testing.T) {
		// A result which fits is returned as it is, measured once
		var calls int
		result := json.RawMessage(`{"city": "Berlin"}`)
		truncated, err := toolkit.TruncateResult(ctx, result, 100, bytesPerToken(4, &calls))
		require.NoError(t, err)
		assert.Equal(t, string(result), string(truncated))
		assert.Equal(t, 1, calls)
	})

	t.Run("NoLimit", func(t *testing.T) {
		// A limit of zero is no limit, so the result isn't measured
		var calls int
		result := json.RawMessage(`"` + strings.Repeat("a", 1000) + `"`)
		truncated, err := toolkit.TruncateResult(ctx, result, 0, bytesPerToken(4, &calls))
		require.NoError(t, err)
		assert.Equal(t, string(result), string(truncated))
		assert.Zero(t, calls)
	})

	t.Run("String", func(t *testing.T) {
		// A long string is cut, with a note of how much more there was, and
		// as much of it as fits
		truncated := truncate(t, `"`+strings.Repeat("a", 10000)+`"`, 500)
		var text string
		require.NoError(t, json.Unmarshal(truncated, &text))
		assert.True(t, strings.HasPrefix(text, strings.Repeat("a", 1800)), text)
		assert.Contains(t, text, "more characters]")
	})

	t.Run("Array", func(t *testing.T) {
		// A long array keeps its first items, with a note of how many more
		// there were
		var items []string
		for i := range 2000 {
			items = append(items, fmt.Sprint(i))
		}
		truncated := truncate(t, "["+strings.Join(items, ",")+"]", 100)
		var values []any
		require.NoError(t, json.Unmarshal(truncated, &values))
		require.Greater(t, len(values), 2)
		assert.Equal(t, json.Number("0"), json.Number(fmt.Sprint(values[0])))
		assert.Regexp(t, `^…\[\d+ more items\]$`, values[len(values)-1])
	})

	t.Run("Object", func(t *testing.T) {
		// The long values of an object are cut, and its short ones, and its
		// keys, are kept, in order
		truncated := truncate(t, `{"id":7,"title":"News","body":"`+strings.Repeat("words ", 2000)+`","ok":true}`, 200)
		assert.Regexp(t, `^\{"id":7,"title":"News","body":"words words .*more characters\]","ok":true\}$`, string(truncated))
	})

	t.Run("Keys", func(t *testing.T) {
		// An object with many keys keeps its first, in order, with a note of
		// how many more there were
		var members []string
		for i := range 1000 {
			members = append(members, fmt.Sprintf(`"key%04d":%d`, i, i))
		}
		truncated := truncate(t, "{"+strings.Join(members, ",")+"}", 100)
		assert.True(t, strings.HasPrefix(string(truncated), `{"key0000":0,"key0001":1,`), string(truncated))
		assert.Regexp(t, `,"…":"\[\d+ more keys\]"\}$`, string(truncated))
	})

	t.Run("Nested", func(t *testing.T) {
		// Arrays of objects with long strings are cut throughout
		var hits []string
		for i := range 100 {
			hits = append(hits, fmt.Sprintf(`{"rank":%d,"snippet":%q}`, i, strings.Repeat("x", 500)))
		}
		truncated := truncate(t, `{"query":"go","hits":[`+strings.Join(hits, ",")+`]}`, 300)
		assert.True(t, strings.HasPrefix(string(truncated), `{"query":"go","hits":[{"rank":0,"snippet":"xxx`), string(truncated))
		assert.Contains(t, string(truncated), "more characters]")
		assert.Contains(t, string(truncated), "more items]")
	})

	t.Run("UTF8", func(t *testing.T) {
		// Strings are cut between characters, and HTML isn't escaped
		truncated := truncate(t, `"<p>`+strings.Repeat("ü€😀", 2000)+`"`, 100)
		assert.True(t, strings.HasPrefix(string(truncated), `"<p>ü€😀`), string(truncated))
	})

	t.Run("NotJSON", func(t *testing.T) {
		// A result which isn't JSON is cut as text, as a JSON string
		truncated := truncate(t, `{"unterminated": "`+strings.Repeat("a", 5000), 100)
		var text string
		require.NoError(t, json.Unmarshal(truncated, &text))
		assert.True(t, strings.HasPrefix(text, `{"unterminated": "aaa`), text)
	})

	t.Run("Deep", func(t *testing.T) {
		// A result whose shape doesn't fit, however short it is, is cut as
		// text, as a JSON string
		truncated := truncate(t, strings.Repeat("[", 500)+strings.Repeat("]", 500), 50)
		var text string
		require.NoError(t, json.Unmarshal(truncated, &text))
		assert.True(t, strings.HasPrefix(text, "[[[["), text)
	})

	t.Run("LeftOut", func(t *testing.T) {
		// A result of which nothing fits is left out, with a note
		truncated, err := toolkit.TruncateResult(ctx, json.RawMessage(`"`+strings.Repeat("a", 1000)+`"`), 1, bytesPerToken(4, nil))
		require.NoError(t, err)
		var text string
		require.NoError(t, json.Unmarshal(truncated, &text))
		assert.Contains(t, text, "was left out")
	})

	t.Run("Underestimated", func(t *testing.T) {
		// A model whose tokens are fewer bytes than the result's suggest is
		// measured again, until the result fits
		var calls int
		measure := func(_ context.Context, text string) (uint, error) {
			calls++
			if calls == 1 {
				return uint(len(text) / 8), nil
			}
			return uint(len(text) / 2), nil
		}
		truncated, err := toolkit.TruncateResult(ctx, json.RawMessage(`"`+strings.Repeat("a", 10000)+`"`), 100, measure)
		require.NoError(t, err)
		assert.True(t, json.Valid(truncated))
		assert.LessOrEqual(t, len(truncated)/2, 100)
		assert.Greater(t, calls, 2)
	})

	t.Run("Error", func(t *testing.T) {
		// The function's error is returned
		_, err := toolkit.TruncateResult(ctx, json.RawMessage(`"text"`), 1, func(context.Context, string) (uint, error) {
			return 0, errors.New("unavailable")
		})
		assert.ErrorContains(t, err, "unavailable")
	})
}
