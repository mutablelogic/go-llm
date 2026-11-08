//go:build client

package main

import (
	// Packages
	broadcastercmd "github.com/mutablelogic/go-llm/broadcaster/cmd"
	heartbeatcmd "github.com/mutablelogic/go-llm/heartbeat/cmd"
	mcpcmd "github.com/mutablelogic/go-llm/mcp/registry/cmd"
	memorycmd "github.com/mutablelogic/go-llm/memory/cmd"
	registrycmd "github.com/mutablelogic/go-llm/registry/cmd"
	resourcecmd "github.com/mutablelogic/go-llm/resource/cmd"
	toolkitcmd "github.com/mutablelogic/go-llm/toolkit/cmd"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// CLI is the client, which has the commands which call a server, but not
// the commands which run a server. It's built with the "client" tag, which
// leaves the server out of the command packages too.
type CLI struct {
	Registry    registrycmd.ClientCommands    `embed:""`
	Toolkit     toolkitcmd.ClientCommands     `embed:""`
	Broadcaster broadcastercmd.ClientCommands `embed:""`
	Memory      memorycmd.ClientCommands      `embed:""`
	Heartbeat   heartbeatcmd.ClientCommands   `embed:""`
	Resource    resourcecmd.ClientCommands    `embed:""`
	Connector   mcpcmd.ClientCommands         `embed:""`
}
