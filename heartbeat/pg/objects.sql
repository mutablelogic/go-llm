
-- heartbeat.heartbeat
CREATE TABLE IF NOT EXISTS ${"schema"}."heartbeat" (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  UUID        NOT NULL REFERENCES ${"llm"}."session" (id) ON DELETE CASCADE,
    message     TEXT        NOT NULL,
    schedule    JSONB       NOT NULL,
    fired       BOOLEAN     NOT NULL DEFAULT FALSE,
    last_fired  TIMESTAMPTZ          DEFAULT NULL,
    created     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified    TIMESTAMPTZ          DEFAULT NULL
);