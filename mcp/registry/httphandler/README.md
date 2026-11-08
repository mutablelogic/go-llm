# MCP Connectors API

## Connector Operations

Connectors are remote MCP servers, which the assistant can connect to, such as for their tools. Each has a `name`, which is an identifier, of up to 64 letters, numbers, underscores or hyphens, starting with a letter, and the `url` of its server, which is an `http` or `https` URL, of its streamable HTTP transport, or its SSE transport. A server which runs as a command, over stdio, can't be a connector. Each connector has a `serial`, which changes each time it's changed, and its status, which isn't stored, as it's the server's: whether its MCP server is `connected`, and when it is, the `server`'s `name` and `version`, as it said, or the `error` of why it couldn't be connected, or its session ended. A connector which is changed, or deleted, is disconnected.

The connectors are kept in the server's store of them, so they're kept as the server restarts, when it has one, and are otherwise lost when it stops.

### GET /mcp/connector

List the connectors, in order of their names, with their `count`.

### POST /mcp/connector

Create a connector, with its `name` and `url`. Returns the created connector, with its `serial`. Returns 400 if the request body is invalid, the name isn't an identifier, or the URL isn't an `http` or `https` URL with a host, or 409 if a connector with that name exists already.

### GET /mcp/connector/{name}

Get a connector by name. Returns 404 if there's no connector with that name.

### PATCH /mcp/connector/{name}

Update a connector by name, replacing its `url`, once the server at the URL is connected to, so a URL which isn't a server's isn't kept, and the connector is left as it was. Its name can't be changed. Returns the updated connector, with its new `serial`, connected, and disconnects the server it was connected to before. Returns 400 if the request body is invalid, or the URL isn't an `http` or `https` URL with a host, 404 if there's no connector with that name, 409 if it was changed as it was updated, or 503 if the server at the URL can't be connected to.

### DELETE /mcp/connector/{name}

Delete a connector by name. Returns the deleted connector. Returns 404 if there's no connector with that name, or 409 if it was changed as it was deleted.
