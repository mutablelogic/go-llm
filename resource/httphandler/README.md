# Resource API

## Resource Operations

A resource is content, such as an image, a document, or audio, which is uploaded, and which models, and tools, can read. Its content is kept once, by its `hash`, which is its SHA-256, in hexadecimal, so the same content, uploaded again, isn't kept twice, and it's referred to as `resource://<hash>`. A resource has its `size`, in bytes, its `name`, such as its file's name, its `description`, its `type`, which is its MIME type, when it was `created`, and when it was last `used`, which is when it was last created again, or its content read.

A resource is in a `session`, or on its own. A resource in a session is only found in that session, with the `session` parameter, and is deleted with it. A message's attachment, in a chat, which refers to a resource on its own, by its URI, links it to the chat's session, as the session's resource, with the one content. A resource on its own is found without one, and is deleted once it hasn't been used for a day, unless the server says otherwise, with `--resource.retention`. The same content can be a resource in more than one session, and on its own, with the one content. Deleting a resource, or its session, only unlinks its content, which is removed when the resources are cleaned, when the server starts, and every hour, once no resource refers to it.

When a resource is created, used again, or deleted, an event is published, whose `schema` is `resource`, whose `table` is `resource`, whose `row` is its hash, and whose data is its `session`, when it has one.

### GET /resource

List the resources on their own, or those in the `session`, the most recently used first, with how many there are. Up to 100 are listed at a time, from the `offset`, and at most the `limit`, which is at most 1000. Returns 400 if the session isn't a UUID.

### POST /resource

Upload a resource, as the request body, with its `Content-Type`, or as a file in a `multipart/form-data` form. What it is, its `session`, `name`, `description` and `type`, are the query's parameters, or the form's values, which are before its file. Its name, when it's omitted, is the file's name, without its directories, and its type, when it's omitted, or is `application/octet-stream`, is the file's, or its name's, or is detected from its content. Its content is kept as it's uploaded, so a large file isn't held in memory.

Returns the resource, with 201 when it's created, or 200 when there's a resource with its content in its session already, which is used again, with its name, description and type, when they're given. Returns 400 if the session isn't a UUID, the type isn't a MIME type, or there's no content, 404 if the session doesn't exist, or 413 if its content is larger than 256 MiB, unless the server says otherwise, with `--resource.max-size`.

### GET /resource/{hash}

Get the resource with the hash, on its own, or in the `session`. Returns 400 if the hash isn't a SHA-256, or the session isn't a UUID, or 404 if there's no resource with the hash, on its own, or in the session.

### GET /resource/{hash}/content

Download the content of the resource with the hash, on its own, or in the `session`, with its type, its size, and its name, as the file's name, which is shown in the browser, and its hash as its `ETag`. Its content is read in ranges, when they're asked for, with `Range`. The resource is used. Returns 400 if the hash isn't a SHA-256, or the session isn't a UUID, or 404 if there's no resource with the hash, on its own, or in the session.

### DELETE /resource/{hash}

Delete the resource with the hash, on its own, or in the `session`. Its content is removed later, when the resources are cleaned, once no resource refers to it. Returns the resource, as it was. Returns 400 if the hash isn't a SHA-256, or the session isn't a UUID, or 404 if there's no resource with the hash, on its own, or in the session.
