
-- heartbeat.insert
INSERT INTO ${"schema"}."heartbeat" (session, message, schedule)
VALUES (@session, @message, @schedule)
RETURNING
    id, message, schedule, fired, last_fired, created, modified

--heartbeat.update
UPDATE ${"schema"}."heartbeat" SET ${patch}
WHERE id = @id
RETURNING
    id, message, schedule, fired, last_fired, created, modified

--heartbeat.list
SELECT
    id, message, schedule, fired, last_fired, created, modified
FROM ${"schema"}."heartbeat" ${where} ORDER BY last_fired ASC NULLS FIRST, created ASC

--heartbeat.list_for_user
SELECT
    heartbeat.id,
    heartbeat.message,
    heartbeat.schedule,
    heartbeat.fired,
    heartbeat.last_fired,
    heartbeat.created,
    heartbeat.modified
FROM ${"schema"}."heartbeat" AS heartbeat
JOIN ${"llm"}."session" AS session ON session.id = heartbeat.session
WHERE session."user" = @user
${where}
ORDER BY heartbeat.last_fired ASC NULLS FIRST, heartbeat.created ASC

--heartbeat.delete
DELETE FROM ${"schema"}."heartbeat" WHERE id = @id
RETURNING
    id, message, schedule, fired, last_fired, created, modified

-- heartbeat.mark_fired
-- The @fired parameter is computed by Go: true if Next() returns zero (no future occurrence).
UPDATE ${"schema"}."heartbeat" SET
    fired      = @fired,
    last_fired = CASE WHEN NOT @fired THEN NOW() ELSE last_fired END
WHERE id = @id
RETURNING
    id, message, schedule, fired, last_fired, created, modified

--heartbeat.select
SELECT
    id, message, schedule, fired, last_fired, created, modified
FROM ${"schema"}."heartbeat"
WHERE id = @id