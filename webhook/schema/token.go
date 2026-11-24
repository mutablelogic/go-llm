package schema

import (
	"regexp"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// escaped is what's before the two hex digits of a character which is
	// escaped, as a URL's is, which can be more than once, such as %25, or as
	// JSON's is, in a string which is JSON
	escaped = `(?:%(?:25)*|\\+u00)`

	// hookPath is the path before the token, any of whose characters can be
	// escaped, as can its slash, as JSON's is
	hookPath = `(?:h|` + escaped + `68)(?:o|` + escaped + `6f){2}(?:k|` + escaped + `6b)(?:\\*/|` + escaped + `2f)`
)

// hookToken is the token of a webhook's URL, any of whose characters can be
// escaped
var hookToken = regexp.MustCompile(`(?i)(` + hookPath + `)(?:[a-z0-9_-]|` + escaped + `[0-9a-f]{2}){43,}`)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// HideToken returns the text, with {token} in the place of the tokens of
// webhooks' URLs, as whoever has one can call its webhook. A token which isn't
// in a URL isn't known to be one.
func HideToken(text string) string {
	return hookToken.ReplaceAllString(text, "${1}{token}")
}
