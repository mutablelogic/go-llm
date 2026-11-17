# Workspaces

## Workspace Operations

Workspaces are sets of folders and resources, such as notes and files. An MCP connector's namespace is a workspace while it's connected to a server which has resources. A workspace's name is its namespace's. Each says whether its resources are `read`, and whether its changes are `watch`ed, which are published as events, in the `workspace` schema. A workspace's resources, and their artwork, have its URI of them, `resource://<workspace>/<uri>`, such as `resource://github/ui://github/get-me`, whose path is the resource's own URI, as its MCP server has it, so a resource is read by its URI alone, wherever it's from. A resource on its own, such as a conversation's, is `resource:///<hash>`, which has no workspace.

### GET /llm/workspace

List the workspaces, in order of name, with each one's `title` and `version`, such as its MCP server's, its `description`, as its namespace says, and whether it reads its resources, and is watched. `count` is the number of workspaces.

### GET /llm/workspace/{name}

Get a workspace by name, with its `title`, `description` and `version`, and whether it reads its resources, and is watched. Returns 404 if there is no workspace with the name, or it isn't available, such as a connector which isn't connected.

### GET /llm/workspace/{name}/resource

List a page of the workspace's folders and resources: those in the folder `path`, or the top, without one, or, with `recursive`, every one under it. Each has its `uri`, `name`, `type`, which is `inode/directory` for a folder, and, when they're known, its `title`, `description`, `size`, `modified` time, `meta`, which is its metadata, by its `ns`, `key`, `text` and JSON `value`, and `artwork`, such as its thumbnail, by its `key`, `type` and `uri`. `limit` is the most to return, and `next` is the `cursor` of the next page, which is left out when there isn't one. A workspace which doesn't list by folder, such as an MCP server which isn't a workspace's, lists all its resources. Returns 404 if there is no workspace with the name.

### GET /llm/workspace/{name}/content

Read a resource of the workspace by its `uri`, as it's listed, `resource://<workspace>/<uri>`, or its own. Its content is returned with its type, and, with `format=markdown`, its text, as a Markdown document, a part at a time, from `offset`, of at most `limit` bytes, whose YAML front matter has its metadata, and the `next` part's offset. Its `ETag` is the SHA-256 of its content, so a request with `If-None-Match`, or `If-Modified-Since`, of it, isn't sent it again, and it can be read in ranges. With `download`, it's an attachment, which a browser saves. Returns 400 if the `uri` is missing, or the `format` isn't `raw` or `markdown`, 404 if there is no workspace with the name, or it has no resource with the URI, or 501 if the workspace doesn't read its resources, or, with `markdown`, the resource has no Markdown, such as audio.
