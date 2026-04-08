
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
-- The @fired parameter is computed by Go: true if Next() returns zero (no future occurrence).
UPDATE ${"schema"}."heartbeat" SET
    fired      = @fired,
    last_fired = CASE WHEN NOT @fired THEN NOW() ELSE last_fired END
WHERE id = @id
RETURNING
    id, message, schedule, fired, last_fired, created, modified, meta

--heartbeat.select
SELECT
    id, message, schedule, fired, last_fired, created, modified, meta
FROM ${"schema"}."heartbeat"
WHERE id = @id