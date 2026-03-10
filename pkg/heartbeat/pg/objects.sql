
-- heartbeat.heartbeat
CREATE TABLE IF NOT EXISTS ${"schema"}."heartbeat" (
    id          UUID        PRIMARY KEY,
    message     TEXT        NOT NULL,
    schedule    JSONB       NOT NULL,
    fired       BOOLEAN     NOT NULL DEFAULT FALSE,
    last_fired  TIMESTAMPTZ,
    created     TIMESTAMPTZ NOT NULL,
    modified    TIMESTAMPTZ NOT NULL,
    meta        JSONB NOT NULL DEFAULT '{}'
);
