
-- heartbeat.heartbeat
CREATE TABLE IF NOT EXISTS ${"schema"}."heartbeat" (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    message     TEXT        NOT NULL,
    schedule    JSONB       NOT NULL,
    fired       BOOLEAN     NOT NULL DEFAULT FALSE,
    last_fired  TIMESTAMPTZ          DEFAULT NULL,
    created     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified    TIMESTAMPTZ          DEFAULT NULL,
    meta        JSONB       NOT NULL DEFAULT '{}'
);