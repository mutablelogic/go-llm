// Package common has what the webhook protocols share: their bodies as JSON
// values, signatures compared in constant time, and a JSON object's fields
package common

import (
	"bytes"
	"crypto/hmac"
	"crypto/subtle"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Decode returns the body as JSON values: as it is, when it's JSON, an
// object of its fields, when it's a form, whose fields with more than one
// value are arrays, a string, when it's other text, or nil, when it's empty,
// or isn't text
func Decode(header http.Header, body []byte) (json.RawMessage, error) {
	if len(body) == 0 {
		return nil, nil
	}
	switch mediaType := MediaType(header); {
	case mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"):
		if !json.Valid(body) {
			return nil, llm.ErrBadParameter.With("body isn't valid JSON")
		}
		return json.RawMessage(body), nil
	case mediaType == "application/x-www-form-urlencoded":
		form, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("body isn't a valid form: %v", err)
		}
		fields := make(map[string]any, len(form))
		for name, values := range form {
			if len(values) == 1 {
				fields[name] = values[0]
			} else {
				fields[name] = values
			}
		}
		return json.Marshal(fields)
	case utf8.Valid(body):
		return json.Marshal(string(body))
	}
	return nil, nil
}

// MediaType returns the media type of the Content-Type header, without its
// parameters, or empty
func MediaType(header http.Header) string {
	mediaType, _, _ := mime.ParseMediaType(header.Get("Content-Type"))
	return mediaType
}

// Equal returns whether the signature, which isn't empty, is the expected
// one, in constant time
func Equal(signature, expected []byte) bool {
	return len(signature) > 0 && hmac.Equal(signature, expected)
}

// EqualString returns whether the strings, the first of which isn't empty,
// are equal, in constant time
func EqualString(a, b string) bool {
	return len(a) > 0 && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// NoSecret is the error of verifying a call of a webhook with no secret
func NoSecret() error {
	return llm.ErrForbidden.With("the webhook has no secret, so its calls can't be verified")
}

// Field returns the field of the JSON object, at the path, such as msg, id,
// as text, when it's a string, or a number, as it was sent, or empty, when
// it isn't one
func Field(values json.RawMessage, path ...string) string {
	decoder := json.NewDecoder(bytes.NewReader(values))
	decoder.UseNumber()
	var v any
	if decoder.Decode(&v) != nil {
		return ""
	}
	for _, name := range path {
		object, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = object[name]
	}
	switch v := v.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	}
	return ""
}
