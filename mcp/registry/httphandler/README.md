# MCP Connectors API

## Connector Operations

Connectors are remote MCP servers, which the assistant can connect to, such as for their tools. Each has a `name`, which is an identifier, of up to 64 letters, numbers, underscores or hyphens, starting with a letter, and the `url` of its server, which is an `http` or `https` URL, of its streamable HTTP transport, or its SSE transport. Its server's tools, prompts and resources are those whose names match one of its `include` patterns, or all of them, when it has none, and none of its `exclude` patterns, which are regular expressions, such as `signup`, as a provider's models are. A server which runs as a command, over stdio, can't be a connector. Each connector has a `serial`, which changes each time it's changed, and its status, which isn't stored, as it's the server's: whether its MCP server is `connected`, and when it is, the `server`'s `name` and `version`, as it said, or the `error` of why it couldn't be connected, or its session ended. A connector which is changed, or deleted, is disconnected. A connector has a `namespace` of its own, in the toolkit, for as long as it exists, which is the connector's name, without what a namespace can't have, and with four random digits added, when it's another namespace's, such as `memory`, or the agents', `agent`. Once its server is connected, its tools are in the namespace, so they're called as `<namespace>:<tool>`, such as `github:search_issues`, and while it isn't, the namespace has no tools.

The server connects to a connector's MCP server as it's created, or as the server starts, and tries again every 30 seconds, when it can't be connected, or its session ends. Its events say when a connector's MCP server is connected, as a `create` of the `connector` table of the `mcp` schema, whose row is its name, with its `serial`, and the `server`'s `name` and `version`, and when it's disconnected, as a `delete`, with the `error` of why, when it wasn't disconnected, such as when its server closed the session.

A connector whose MCP server needs to be authorized, as it answers `401 Unauthorized`, has `auth` of `required`, and its event is an `update`, with `auth` of `required`, and the `error`. It's signed in to with `POST /mcp/connector/{name}/authorize`, which returns the `url` a person signs in at, with the authorization code flow of OAuth 2.1, and PKCE. Once they have, their browser is redirected to `GET /mcp/callback`, and the server exchanges its code for a token, which it keeps, and refreshes, and connects the connector, whose `auth` is then `authorized`. The server registers itself as a client with the server's authorization server, dynamically, unless the connector has a `client_id_credential`, as some authorization servers, such as GitHub's, don't register clients. That's the name of the credential holding the client's identifier, such as `GITHUB_CLIENT_ID`, and its `client_secret_credential` is the name of the one holding its secret, such as `GITHUB_CLIENT_SECRET`, which the server's credentials provider has, such as its environment variables, so they're got each time they're used, and the identifier, and secret, themselves, aren't kept. A connector's token is only sent to the server it was signed in to, with its client, so it's deleted when the connector's `url`, or `client_id_credential`, is changed.

The connectors are kept in the server's store of them, so they're kept as the server restarts, when it has one, and are otherwise lost when it stops.

### GET /mcp/connector

List the connectors, in order of their names, with their `count`.

### POST /mcp/connector

Create a connector, with its `url`, its `name`, which is the server's own, as it says once it's connected, or as the metadata of how it's authorized says, when it needs to be, or else the host of its URL, when it's omitted, as an identifier, such as `github-mcp-server`, with a number, such as `github-mcp-server-2`, when that's another connector's, its `include` and `exclude` patterns, and its `client_id_credential` and `client_secret_credential`, when it has them. Returns the created connector, with its `serial`. Returns 400 if the request body is invalid, the name isn't an identifier, a credential doesn't exist, or a secret's credential is given without an identifier's, the URL isn't an `http` or `https` URL with a host, or a pattern isn't a regular expression, or 409 if a connector with that name exists already.

### GET /mcp/connector/{name}

Get a connector by name. Returns 404 if there's no connector with that name.

### PATCH /mcp/connector/{name}

Update a connector by name, replacing its `url`, its `include` and `exclude` patterns, and its `client_id_credential` and `client_secret_credential`, so a field which is omitted is cleared, once the server at the URL is connected to, when the URL, or `client_id_credential`, is another, so a URL which isn't a server's isn't kept, and the connector is left as it was. When they're the same, such as when only its patterns change, it isn't connected to first: it keeps its session, when it's connected, or is connected again, as it is now. Its name can't be changed. Returns the updated connector, with its new `serial`, connected, and disconnects the server it was connected to before. Returns 400 if the request body is invalid, the URL isn't an `http` or `https` URL with a host, or a pattern isn't a regular expression, 404 if there's no connector with that name, 409 if it was changed as it was updated, or 503 if the server at the URL can't be connected to. A server which needs to be authorized is kept, with `auth` of `required`, rather than returning 503.

### DELETE /mcp/connector/{name}

Delete a connector by name. Returns the deleted connector. Returns 404 if there's no connector with that name, or 409 if it was changed as it was deleted.

### POST /mcp/connector/{name}/authorize

Start signing in to a connector's MCP server, which returns the `url` a person signs in at, in their browser, within 10 minutes. The request's `redirect` is where the authorization server redirects their browser, once they have, which the web UI sets to this API's `GET /mcp/callback`, at the page's origin. When it's omitted, it's the callback at the server's own URL, when that's `https`, as it is when the server is given its public name, such as that of a proxy it's behind, with `--tls.name`, and otherwise at the request's `Origin`, as a browser sends it. A command line client, which reaches the server at another address than a person's browser does, sends the callback of the address it reaches it at, or a loopback address of its own, which passes the code on to the callback. Returns 400 if the redirect isn't an `http` or `https` URL, or it's omitted, and the request has no origin, 404 if there's no connector with that name, 501 if its authorization server doesn't register clients, and the connector has no `client_id_credential`, or 503 if its authorization server can't be discovered.

### GET /mcp/callback

The callback which an authorization server redirects a person's browser to, once they've signed in, with the `state` of the authorization, and its `code`, or the `error` of why they didn't, which exchanges the code for a token, and connects the connector. Returns a page, for the person, which says whether they're signed in, or the connector, as JSON, when the request accepts `application/json`, rather than HTML, such as from a command line client's loopback. Returns 400 if they didn't sign in, or the code couldn't be exchanged, 404 if no authorization is waited for, with the state, such as when it's been 10 minutes, or the connector was deleted, or 409 if the connector's `url`, or `client_id_credential`, was changed as they signed in.

### GET /mcp/template

List the templates of the remote MCP servers which a connector can be added for, without typing their URL, such as GitHub's, in order of their names, with their `count`. Each has the server's `name`, its `url`, its `icon`, which is the file name of its logo, among the web UI's icons at `icon/connector/`, and the names of the credentials of the client it's signed in to with, `client_id_credential` and `client_secret_credential`, when its sign-in doesn't register clients itself. The server offers those of its own list, whose credentials it has, such as its environment variables `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET`, as it starts, apart from those whose `url` is a connector's already. A connector is added for one with `POST /mcp/connector`, with its `url`, and credentials' names.
