# MCP Connectors API

## Connector Operations

Connectors are remote MCP servers, which the assistant can connect to, such as for their tools. Each has a `name`, which is an identifier, of up to 64 letters, numbers, underscores or hyphens, starting with a letter, and the `url` of its server, which is an `http` or `https` URL, of its streamable HTTP transport, or its SSE transport. Its server's tools, prompts and resources are those whose names match one of its `include` patterns, or all of them, when it has none, and none of its `exclude` patterns, which are regular expressions, such as `signup`, as a provider's models are. A server which runs as a command, over stdio, can't be a connector. Each connector has a `serial`, which changes each time it's changed, and its status, which isn't stored, as it's the server's: whether its MCP server is `connected`, and when it is, the `server`'s `name` and `version`, as it said, or the `error` of why it couldn't be connected, or its session ended. A connector which is changed, or deleted, is disconnected. A connector has a `namespace` of its own, in the toolkit, for as long as it exists, which is the connector's name, without what a namespace can't have, and with four random digits added, when it's another namespace's, such as `memory`, or the agents', `agent`. Once its server is connected, its tools are in the namespace, so they're called as `<namespace>:<tool>`, such as `github:search_issues`, and while it isn't, the namespace has no tools.

The server connects to a connector's MCP server as it's created, or as the server starts, and tries again every 30 seconds, when it can't be connected, or its session ends. Its events say when a connector's MCP server is connected, as a `create` of the `connector` table of the `mcp` schema, whose row is its name, with its `serial`, and the `server`'s `name` and `version`, and when it's disconnected, as a `delete`, with the `error` of why, when it wasn't disconnected, such as when its server closed the session.

A connector whose MCP server needs to be authorized, as it answers `401 Unauthorized`, has `auth` of `required`, and its event is an `update`, with `auth` of `required`, and the `error`. It's signed in to with `POST /mcp/connector/{name}/authorize`, which returns the `url` a person signs in at, with the authorization code flow of OAuth 2.1, and PKCE. Once they have, their browser is redirected to `GET /mcp/callback`, and the server exchanges its code for a token, which it keeps, and refreshes, and connects the connector, whose `auth` is then `authorized`. The server registers itself as a client with the server's authorization server, dynamically, unless the connector has a `client_id`, as some authorization servers, such as GitHub's, don't register clients. Its `client_secret` is a credential's placeholder, such as `${GITHUB_CLIENT_SECRET}`, which is the server's environment variable, so the secret itself isn't kept, and isn't returned.

The connectors are kept in the server's store of them, so they're kept as the server restarts, when it has one, and are otherwise lost when it stops.

### GET /mcp/connector

List the connectors, in order of their names, with their `count`.

### POST /mcp/connector

Create a connector, with its `url`, its `name`, which is the server's own, as it says once it's connected, or as the metadata of how it's authorized says, when it needs to be, or else the host of its URL, when it's omitted, as an identifier, such as `github-mcp-server`, with a number, such as `github-mcp-server-2`, when that's another connector's, its `include` and `exclude` patterns, and its `client_id` and `client_secret`, when it has them. Returns the created connector, with its `serial`. Returns 400 if the request body is invalid, the name isn't an identifier, the URL isn't an `http` or `https` URL with a host, or a pattern isn't a regular expression, or 409 if a connector with that name exists already.

### GET /mcp/connector/{name}

Get a connector by name. Returns 404 if there's no connector with that name.

### PATCH /mcp/connector/{name}

Update a connector by name, replacing its `url`, its `include` and `exclude` patterns, and its `client_id`, so a field which is omitted is cleared, apart from its `client_secret`, which is kept, as it isn't returned, unless its `client_id` is changed, once the server at the URL is connected to, so a URL which isn't a server's isn't kept, and the connector is left as it was. Its name can't be changed. Returns the updated connector, with its new `serial`, connected, and disconnects the server it was connected to before. Returns 400 if the request body is invalid, the URL isn't an `http` or `https` URL with a host, or a pattern isn't a regular expression, 404 if there's no connector with that name, 409 if it was changed as it was updated, or 503 if the server at the URL can't be connected to. A server which needs to be authorized is kept, with `auth` of `required`, rather than returning 503.

### DELETE /mcp/connector/{name}

Delete a connector by name. Returns the deleted connector. Returns 404 if there's no connector with that name, or 409 if it was changed as it was deleted.

### POST /mcp/connector/{name}/authorize

Start signing in to a connector's MCP server, which returns the `url` a person signs in at, in their browser, within 10 minutes. The request's `redirect` is where the authorization server redirects their browser, once they have, which is this API's `GET /mcp/callback`, of the request's `Origin`, as a browser sends, when it's omitted, so the server needs no public URL of its own, and it can be signed in to from wherever it's reached. A command line client, which reaches the server at another address than a person's browser does, sends the callback of the address it reaches it at, or a loopback address of its own, which passes the code on to the callback. Returns 400 if the redirect isn't an `http` or `https` URL, or it's omitted, and the request has no origin, 404 if there's no connector with that name, 501 if its authorization server doesn't register clients, and the connector has no `client_id`, or 503 if its authorization server can't be discovered.

### GET /mcp/callback

The callback which an authorization server redirects a person's browser to, once they've signed in, with the `state` of the authorization, and its `code`, or the `error` of why they didn't, which exchanges the code for a token, and connects the connector. Returns a page, for the person, which says whether they're signed in, or the connector, as JSON, when the request accepts `application/json`, rather than HTML, such as from a command line client's loopback. Returns 400 if they didn't sign in, or the code couldn't be exchanged, or 404 if no authorization is waited for, with the state, such as when it's been 10 minutes.
