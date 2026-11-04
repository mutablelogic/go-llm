package toolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// MeasureFn returns how many tokens the text is, for the model a tool's
// result is sent to
type MeasureFn func(ctx context.Context, text string) (uint, error)

// node is a JSON value, whose objects keep the order of their keys
type node struct {
	kind  kind
	keys  []string        // The keys of an object
	items []node          // The values of an object, or the items of an array
	str   string          // A string
	raw   json.RawMessage // A number, true, false or null
}

// kind is the kind of a JSON value
type kind int

const (
	kindObject kind = iota
	kindArray
	kindString
	kindRaw
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// truncateMargin is the fraction of the bytes, estimated from the tokens,
	// a result is truncated to, as the estimate can be over
	truncateMargin = 0.95

	// truncateAttempts is how many times a result is truncated, and measured
	// again, before it's left out
	truncateAttempts = 4

	// itemsPerCharacter is how many fewer items of arrays and objects a
	// result is truncated to, than characters of its strings
	itemsPerCharacter = 16

	// truncateKey is the key of the member of an object which says how many
	// of its members were left out
	truncateKey = "…"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// TruncateResult returns the result of a tool, as JSON, so it's at most the
// limit of tokens, as the function measures them, so a model is sent no more
// of a tool's result than fits in its context. A result which fits, or when
// the limit is zero, is returned as it is. Otherwise, its longest strings,
// arrays and objects are shortened, as little as fits, with notes of what
// was left out of each, so the result is valid JSON of the same shape, or,
// when that doesn't fit, or it isn't JSON, it's the start of the result as
// text, as a JSON string, with a note of what was left out of it, or, when
// even that doesn't fit, a note that it was left out. Returns the
// function's error, when it fails.
func TruncateResult(ctx context.Context, result json.RawMessage, limit uint, measure MeasureFn) (json.RawMessage, error) {
	if limit == 0 || len(result) == 0 {
		return result, nil
	}
	tokens, err := measure(ctx, string(result))
	if err != nil {
		return nil, err
	} else if tokens <= limit {
		return result, nil
	}

	// Truncate the result to the bytes the limit is estimated to be, and
	// measure it, until it fits, with fewer bytes each time
	value, parseErr := parse(result)
	budget := scale(len(result), limit, tokens, truncateMargin)
	for range truncateAttempts {
		var truncated json.RawMessage
		if parseErr == nil {
			truncated = value.truncate(budget)
		}
		if truncated == nil {
			truncated = truncateText(result, budget)
		}
		if truncated == nil {
			break
		}
		if tokens, err = measure(ctx, string(truncated)); err != nil {
			return nil, err
		} else if tokens <= limit {
			return truncated, nil
		}
		budget = scale(budget, limit, tokens, truncateMargin*truncateMargin)
	}

	// Leave out the result, as no part of it fits
	return encodeString(fmt.Sprintf("[the result, of %d bytes, was left out, as it's too long]", len(result))), nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// truncate returns the value as JSON of at most the bytes, with its strings,
// arrays and objects shortened, as little as fits, or nil when it doesn't
// fit, however short they are. The most characters of a string, and the most
// items of an array or object, are found by bisection, as the longer they
// can be, the longer the JSON is, so the longest which fit are found.
func (n node) truncate(budget int) json.RawMessage {
	fits := func(chars int) json.RawMessage {
		if data := n.encode(chars, max(1, chars/itemsPerCharacter)); len(data) <= budget {
			return data
		}
		return nil
	}
	best := fits(1)
	if best == nil {
		return nil
	}
	low, high := 1, max(1, n.longest())
	for low < high {
		mid := low + (high-low+1)/2
		if data := fits(mid); data != nil {
			best, low = data, mid
		} else {
			high = mid - 1
		}
	}
	return best
}

// longest returns the most characters of the value's strings, or items of
// its arrays and objects, as characters, which is the most characters it
// can be truncated to, which leave it as it is
func (n node) longest() int {
	result := 0
	switch n.kind {
	case kindString:
		result = utf8.RuneCountInString(n.str)
	case kindObject, kindArray:
		result = len(n.items) * itemsPerCharacter
		for _, item := range n.items {
			result = max(result, item.longest())
		}
	}
	return result
}

// encode returns the value as JSON, with its strings shortened to the
// characters, and its arrays and objects shortened to the items, each with a
// note of what was left out of it
func (n node) encode(chars, items int) json.RawMessage {
	var buf bytes.Buffer
	n.write(&buf, chars, items)
	return buf.Bytes()
}

func (n node) write(buf *bytes.Buffer, chars, items int) {
	switch n.kind {
	case kindRaw:
		buf.Write(n.raw)
	case kindString:
		buf.Write(encodeString(truncateString(n.str, chars)))
	case kindArray:
		buf.WriteByte('[')
		for i, item := range n.items[:min(len(n.items), items)] {
			if i > 0 {
				buf.WriteByte(',')
			}
			item.write(buf, chars, items)
		}
		if more := len(n.items) - items; more > 0 {
			buf.WriteByte(',')
			buf.Write(encodeString(fmt.Sprintf("…[%d more items]", more)))
		}
		buf.WriteByte(']')
	case kindObject:
		buf.WriteByte('{')
		for i, item := range n.items[:min(len(n.items), items)] {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.Write(encodeString(n.keys[i]))
			buf.WriteByte(':')
			item.write(buf, chars, items)
		}
		if more := len(n.items) - items; more > 0 {
			buf.WriteByte(',')
			buf.Write(encodeString(truncateKey))
			buf.WriteByte(':')
			buf.Write(encodeString(fmt.Sprintf("[%d more keys]", more)))
		}
		buf.WriteByte('}')
	}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// parse returns the JSON as a value, whose objects keep the order of their
// keys, and whose numbers are as they are
func parse(data json.RawMessage) (node, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := parseValue(decoder)
	if err != nil {
		return node{}, err
	} else if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return node{}, errors.New("more than one value")
	}
	return value, nil
}

func parseValue(decoder *json.Decoder) (node, error) {
	token, err := decoder.Token()
	if err != nil {
		return node{}, err
	}
	switch token := token.(type) {
	case json.Delim:
		value := node{kind: kindArray}
		if token == '{' {
			value.kind = kindObject
		}
		for decoder.More() {
			if value.kind == kindObject {
				key, err := decoder.Token()
				if err != nil {
					return node{}, err
				}
				value.keys = append(value.keys, key.(string))
			}
			item, err := parseValue(decoder)
			if err != nil {
				return node{}, err
			}
			value.items = append(value.items, item)
		}
		if _, err := decoder.Token(); err != nil {
			return node{}, err
		}
		return value, nil
	case string:
		return node{kind: kindString, str: token}, nil
	case json.Number:
		return node{kind: kindRaw, raw: json.RawMessage(token)}, nil
	case bool:
		if token {
			return node{kind: kindRaw, raw: json.RawMessage("true")}, nil
		}
		return node{kind: kindRaw, raw: json.RawMessage("false")}, nil
	default:
		return node{kind: kindRaw, raw: json.RawMessage("null")}, nil
	}
}

// truncateText returns the start of the result, as text, as a JSON string of
// at most the bytes, with a note of what was left out of it, or nil when no
// part of it fits
func truncateText(result json.RawMessage, budget int) json.RawMessage {
	text := string(result)
	for chars := min(budget, utf8.RuneCountInString(text)); chars > 0; {
		data := encodeString(truncateString(text, chars))
		if len(data) <= budget {
			return data
		}
		chars -= max(1, len(data)-budget)
	}
	return nil
}

// truncateString returns the string, when it's at most the characters, or
// its first characters, with a note of how many more there were
func truncateString(text string, chars int) string {
	if utf8.RuneCountInString(text) <= chars {
		return text
	}
	end := 0
	for range chars {
		_, size := utf8.DecodeRuneInString(text[end:])
		end += size
	}
	return text[:end] + fmt.Sprintf("…[truncated, %d more characters]", utf8.RuneCountInString(text[end:]))
}

// encodeString returns the string as JSON, without escaping HTML, so it's no
// longer than it needs to be
func encodeString(text string) json.RawMessage {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(text)
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// scale returns the bytes, scaled by the limit of tokens to the tokens they
// are, and the margin
func scale(bytes int, limit, tokens uint, margin float64) int {
	return int(float64(bytes) * float64(limit) / float64(tokens) * margin)
}
