// Package schema has the defaults, and types, of MCP clients, of remote
// servers
package schema

import (
	// Packages
	version "github.com/mutablelogic/go-server/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// DefaultName is the name of a client, which a server is told, as it
	// connects, unless it's given another
	DefaultName = "go-llm"
)

var (
	// DefaultVersion is the version of a client, which a server is told, as
	// it connects, unless it's given another, which is the build's
	DefaultVersion = version.Version()
)
