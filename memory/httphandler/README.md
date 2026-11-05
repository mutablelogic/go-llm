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
