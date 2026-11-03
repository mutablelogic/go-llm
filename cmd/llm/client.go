//go:build client

package main

import (
	// Packages
	registrycmd "github.com/mutablelogic/go-llm/registry/cmd"
	toolkitcmd "github.com/mutablelogic/go-llm/toolkit/cmd"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// CLI is the client, which has the commands which call a server, but not
// the commands which run a server. It's built with the "client" tag, which
// leaves the server out of the command packages too.
type CLI struct {
	Registry registrycmd.ClientCommands `embed:""`
	Toolkit  toolkitcmd.ClientCommands  `embed:""`
}
