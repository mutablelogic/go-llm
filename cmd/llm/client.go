//go:build client

package main

import (
	// Packages
	registrycmd "github.com/mutablelogic/go-llm/registry/cmd"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// CLI is the client, which has the commands which call a server, but not
// the commands which run a server. It's built with the "client" tag, which
// leaves the server out of the command packages too.
type CLI struct {
	Registry registrycmd.ClientCommands `embed:""`
}
