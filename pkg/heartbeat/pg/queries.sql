
-- heartbeat.insert
INSERT INTO ${"schema"}."heartbeat" (message, schedule)
VALUES (@message, @schedule)
RETURNING
    id, message, schedule, fired, last_fired, created, modified, meta

--heartbeat.update
UPDATE ${"schema"}."heartbeat" SET ${patch}
WHERE id = @id
RETURNING
    id, message, schedule, fired, last_fired, created, modified, meta

--heartbeat.list
SELECT
    id, message, schedule, fired, last_fired, created, modified, meta
FROM ${"schema"}."heartbeat" ${where} ORDER BY last_fired ASC NULLS FIRST, created ASC

--heartbeat.delete
DELETE FROM ${"schema"}."heartbeat" WHERE id = @id
RETURNING
    id, message, schedule, fired, last_fired, created, modified, meta

-- heartbeat.mark_fired
UPDATE ${"schema"}."heartbeat" SET
    fired      = (schedule->>'year') IS NOT NULL,
    last_fired = CASE WHEN (schedule->>'year') IS NULL THEN NOW() ELSE last_fired END,
    modified   = NOW()
WHERE id = @id
RETURNING
    id, message, schedule, fired, last_fired, created, modified, meta

--heartbeat.select
SELECT
    id, message, schedule, fired, last_fired, created, modified, meta
FROM ${"schema"}."heartbeat"
WHERE id = @id
