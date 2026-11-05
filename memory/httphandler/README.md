# Memory API

## Detail Operations

The memory keeps details of the person the assistant talks to, and of the assistant, which are shared by every session and agent, and are in the system prompt of the `memory` namespace. The fields are `name`, the person's name, `assistant`, the assistant's name, `location`, where the person is, `timezone`, their time zone, by its IANA name, such as `Europe/Berlin`, and `language`, the language they speak.

Each detail is not known, when the details don't have it, or has a `value`, such as `{"value":"Berlin"}`, or is declined, as `{"declined":true}`, when the person doesn't want it known, so it isn't in the system prompt, or asked for.

When a detail changes, an event is published, whose `schema` is `memory`, whose `table` is `detail`, whose `row` is the field, and whose `data` is the detail, when there is one: it's created when it becomes known, or declined, updated when it changes, and deleted when it's forgotten.

### GET /memory/detail

Get the details, which are those which are known, or declined, such as `{"name":{"value":"David"},"location":{"declined":true}}`.

### PUT /memory/detail/{field}

Set the detail of the field, which is its `value`, or that it's `declined`, in the request body. Returns the details. Returns 400 if the field isn't one of the fields, the detail has a value and is declined, or has neither, or its value isn't valid, such as a time zone which isn't known.

### DELETE /memory/detail/{field}

Forget the detail of the field, so it isn't known. Returns the details. Returns 400 if the field isn't one of the fields.

## Memory Operations

Memories are what the assistant remembers, beyond the details, which every session and agent shares, and searches with the `memories` tool of the `memory` namespace. Each memory has a `key`, which is an identifier, such as `kitchen_lights`, a one-line `description`, which says what's in it, a `body`, as Markdown, and a `version`, which is one when it's remembered, and one more each time it's changed, so a change is made to the memory as it was last seen.

When a memory changes, an event is published, whose `schema` is `memory`, whose `table` is `memory`, and whose `row` is its key: it's created when it's remembered, updated when it's changed, including when a line of it is forgotten, and deleted when it's forgotten, or nothing is left of it. Unless it was deleted, its `data` is its new `version`, such as `{"version":2}`, so a client which has that version has seen the change.

### GET /memory/memory

List the memories, the most recently changed first, with how many there are. With `q`, only the memories with each of its words, in their key, description or body, in any case, are listed. Up to 100 are listed at a time, from the `offset`, and at most the `limit`.

### GET /memory/memory/{key}

Get the memory with the key. Returns 404 if there's none.

### PUT /memory/memory/{key}

Remember the memory with the key, which is its `description` and `body`, in the request body, which replace all of what was remembered under it. A new memory has no `version`, and a change to a memory has its `version`, so it isn't made to a memory which was changed since it was seen. Returns the memory, which is 201 when it's created. Returns 400 if the key isn't an identifier, or the memory has no description, a description of more than one line, or no body, or 409 if the version isn't the memory's version, or there's a memory with the key, and no version.

### DELETE /memory/memory/{key}

Forget the memory with the key, or, with a `line`, only the lines of its body which are the line, apart from the space around them, and a bullet, such as `- `, and the memory, when there's nothing left of it. Returns the memory, as it is, or was, when it's forgotten. Returns 404 if there's no memory with the key, or it hasn't the line.
