# Workspaces

## Workspace Operations

Workspaces are sets of folders and resources, such as notes and files. An MCP connector's namespace is a workspace while it's connected to a server which has resources. A workspace's name is its namespace's. Each says whether its resources are `read`, and whether its changes are `watch`ed, which are published as events, in the `workspace` schema.

### GET /llm/workspace

List the workspaces, in order of name, with whether each reads its resources, and is watched. `count` is the number of workspaces.

### GET /llm/workspace/{name}

Get a workspace by name, with whether it reads its resources, and is watched. Returns 404 if there is no workspace with the name, or it isn't available, such as a connector which isn't connected.

### GET /llm/workspace/{name}/resource

List a page of the workspace's folders and resources: those in the folder `path`, or the top, without one, or, with `recursive`, every one under it. Each has its `uri`, `name`, `type`, which is `inode/directory` for a folder, and, when they're known, its `title`, `description`, `size`, `modified` time, `meta`, which is its metadata, by its `ns`, `key`, `text` and JSON `value`, and `artwork`, such as its thumbnail, by its `key`, `type` and `uri`. `limit` is the most to return, and `next` is the `cursor` of the next page, which is left out when there isn't one. A workspace which doesn't list by folder, such as an MCP server which isn't a workspace's, lists all its resources. Returns 404 if there is no workspace with the name.
