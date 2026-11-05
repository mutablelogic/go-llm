# Heartbeat API

## Heartbeat Operations

A heartbeat is a prompt for a session, which fires once, `at` a time, with its offset, such as `2026-11-06T15:00:00+01:00`, or again and again, on a `schedule`, which is a cron expression of five fields, minute, hour, day, month and weekday, such as `0 8 * * 1-5` for eight in the morning on weekdays. A schedule is in its `timezone`, by its IANA name, such as `Europe/Berlin`, or, without one, in the person's, which is the memory's, so it follows them, as it changes, or in UTC, when it isn't known.

Each heartbeat has a `title`, which says what it's for, a `prompt`, which is the instruction a model is given when it fires, and the `session` it's for. It has when it last `fired`, and when it fires `next`, unless it won't, as it fired once. When the clocks go forward, a time which they skip happens once, after they change, and when they go back, a time which is repeated happens once, the first time. A heartbeat which was due when the server wasn't running fires once, when the server starts.

When a heartbeat changes, an event is published, whose `schema` is `heartbeat`, whose `table` is `heartbeat`, and whose `row` is its identifier: it's created, updated, including when it fires, or deleted, including when its session is deleted. When it fires, an event is created, whose `schema` is `heartbeat`, whose `table` is `fire`, whose `row` is its identifier, and whose `data` is its `heartbeat`, `title`, `prompt` and `session`, when it was `scheduled`, and when it `fired`, which is later, when it's late.

### GET /heartbeat

List the heartbeats, those which fire soonest first, and those which won't fire again last, with how many there are. With `session`, only those for the session are listed. Up to 100 are listed at a time, from the `offset`, and at most the `limit`.

### POST /heartbeat

Create a heartbeat, with its `title`, `prompt` and `session`, and either a `schedule`, and a `timezone`, when it has one, or a time, `at`. Returns the heartbeat, with when it fires `next`. Returns 400 if it has no title, or prompt, or session, or its session doesn't exist, has neither a schedule nor a time, or both, its schedule isn't valid, its time zone isn't known, or its time has passed.

### GET /heartbeat/{id}

Get the heartbeat with the identifier. Returns 404 if there's none.

### PATCH /heartbeat/{id}

Change the heartbeat with the identifier: its `title`, `prompt`, `schedule`, which replaces its time, `at`, which replaces its schedule, and fires again, or `timezone`, which is empty to follow the person's. Returns the heartbeat. Returns 400 if it isn't valid, or a new time has passed, or 404 if there's none.

### DELETE /heartbeat/{id}

Delete the heartbeat with the identifier. Returns the heartbeat, as it was. Returns 404 if there's none.
