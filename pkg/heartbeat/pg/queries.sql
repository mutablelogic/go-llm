
-- heartbeat.insert
INSERT INTO ${"schema"}."heartbeat" (
    id, message, schedule, fired, last_fired, created, modified, meta
) VALUES (
    @id, @message, @schedule, DEFAULT, DEFAULT, @created, @modified, DEFAULT
) RETURNING 
    id, message, schedule, fired, last_fired, created, modified, meta

--heartbeat.update
UPDATE ${"schema"}."heartbeat" SET
    message = @message,
    schedule = @schedule,
    fired = @fired,
    last_fired = @last_fired,
    modified = @modified,
    meta = @meta
WHERE id = @id
RETURNING
    id, message, schedule, fired, last_fired, created, modified, meta


--heartbeat.delete
DELETE FROM ${"schema"}."heartbeat" WHERE id = @id
RETURNING
    id, message, schedule, fired, last_fired, created, modified, meta


--heartbeat.select
SELECT
    id, message, schedule, fired, last_fired, created, modified, meta
FROM ${"schema"}."heartbeat"
WHERE id = @id

--heartbeat.list
SELECT
    id, message, schedule, fired, last_fired, created, modified, meta
FROM ${"schema"}."heartbeat"
WHERE ${where}
ORDER BY created DESC
