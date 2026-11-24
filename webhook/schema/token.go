package schema

import (
	"regexp"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// hookToken is the token of a webhook's URL. The slash before it can be
// escaped, as a URL's is, in another URL, more than once, or as JSON's is, in
// a string which is JSON.
var hookToken = regexp.MustCompile(`(?i)(hook(?:\\*/|\\+u002f|%(?:25)*2f))[a-z0-9_-]{43,}`)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// HideToken returns the text, with {token} in the place of the tokens of
// webhooks' URLs, as whoever has one can call its webhook. A token which isn't
// in a URL isn't known to be one.
func HideToken(text string) string {
	return hookToken.ReplaceAllString(text, "${1}{token}")
}
