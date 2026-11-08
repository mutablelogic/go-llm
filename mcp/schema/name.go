package schema

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// DefaultConnectorName is a connector's name, when nothing of what it's
	// named after can be one
	DefaultConnectorName = "mcp"

	// maxConnectorName is the longest name a connector is given, which
	// leaves room for a number, when it's another connector's
	maxConnectorName = 48
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// ConnectorName returns a connector's name, from its server's name, such as
// github for "GitHub", or github-mcp-server for "github-mcp-server", in
// lower case, with each run of what an identifier can't have, such as
// spaces and dots, as a hyphen, starting with a letter, or
// DefaultConnectorName, when nothing's left of it
func ConnectorName(name string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r <= unicode.MaxASCII && (unicode.IsLetter(r) || (b.Len() > 0 && (unicode.IsDigit(r) || r == '_'))):
			if hyphen && b.Len() > 0 {
				b.WriteRune('-')
			}
			hyphen = false
			b.WriteRune(r)
		default:
			hyphen = true
		}
	}
	result := b.String()
	if len(result) > maxConnectorName {
		result = strings.TrimRight(result[:maxConnectorName], "-_")
	}
	if result == "" {
		return DefaultConnectorName
	}
	return result
}

// HostName returns the name of a server, from the host of its URL, such as
// seolinkmap for https://seolinkmap.com/mcp, or mcp.example for
// https://mcp.example.com/mcp, without www, and its top-level domain, for
// when the server can't say its own name
func HostName(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	var labels []string
	for label := range strings.SplitSeq(strings.ToLower(u.Hostname()), ".") {
		if label != "" && label != "www" {
			labels = append(labels, label)
		}
	}
	if len(labels) > 1 {
		labels = labels[:len(labels)-1]
	}
	return strings.Join(labels, ".")
}

// NumberedName returns the name, with a number, when it's another
// connector's, such as github-2, for the second, and the name itself for
// the first
func NumberedName(name string, n int) string {
	if n <= 1 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, n)
}
