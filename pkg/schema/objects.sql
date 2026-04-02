-- llm.message_result
DO $$ BEGIN
  CREATE TYPE ${"schema"}.MESSAGE_RESULT AS ENUM ('stop', 'max_tokens', 'blocked', 'tool_call', 'error', 'other', 'max_iterations');
EXCEPTION
  WHEN duplicate_object THEN null;
END $$;

-- llm.provider
CREATE TABLE IF NOT EXISTS ${"schema"}.provider (
    "name"        TEXT NOT NULL PRIMARY KEY CHECK ("name" ~ '^[a-zA-Z][a-zA-Z0-9_-]{0,63}$'),
    "provider"    TEXT NOT NULL CHECK ("provider" ~ '^[a-zA-Z][a-zA-Z0-9_-]{0,63}$'),
    "url"         TEXT NOT NULL DEFAULT '',
    "enabled"     BOOLEAN NOT NULL DEFAULT true,
    "created_at"  TIMESTAMPTZ NOT NULL DEFAULT now(),
    "modified_at" TIMESTAMPTZ,
    "pv"          INT NOT NULL DEFAULT 0,
    "credentials" BYTEA NOT NULL,
    "include"     TEXT[] NOT NULL DEFAULT '{}',
    "exclude"     TEXT[] NOT NULL DEFAULT '{}',
    "meta"        JSONB
);

-- llm.provider_group
CREATE TABLE IF NOT EXISTS ${"schema"}.provider_group (
  "provider"   TEXT NOT NULL REFERENCES ${"schema"}.provider ("name") ON DELETE CASCADE,
  "group"      TEXT NOT NULL REFERENCES ${"auth"}."group" ("name") ON DELETE CASCADE,
  PRIMARY KEY ("provider", "group")
);

-- llm.session
CREATE TABLE IF NOT EXISTS ${"schema"}.session (
    "id"          UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    "user"        UUID NOT NULL REFERENCES ${"auth"}."user" (id) ON DELETE CASCADE,
    "created_at"  TIMESTAMPTZ NOT NULL DEFAULT now(),
    "modified_at" TIMESTAMPTZ
);

-- llm.message
CREATE TABLE IF NOT EXISTS ${"schema"}.message (
    "id"          UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    "session"     UUID NOT NULL REFERENCES ${"schema"}."session" (id) ON DELETE CASCADE,
    "role"        TEXT NOT NULL,
    "content"     JSONB NOT NULL,
    "tokens"      INT,
    "result"      ${"schema"}.MESSAGE_RESULT,
    "meta"        JSONB,
    "created_at"  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- llm.notify.function
CREATE OR REPLACE FUNCTION ${"schema"}.notify_table()
RETURNS trigger AS $$
DECLARE
  lock_id BIGINT;
BEGIN
  lock_id := hashtextextended(TG_TABLE_SCHEMA || '.' || TG_TABLE_NAME, 0);
  IF pg_try_advisory_xact_lock(lock_id) THEN
    PERFORM pg_notify(
      ${'channel'},
      json_build_object(
        'schema', TG_TABLE_SCHEMA,
        'table', TG_TABLE_NAME,
        'action', TG_OP
      )::text
    );
  END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- llm.notify.provider.trigger
DO $$ BEGIN
  DROP TRIGGER IF EXISTS provider_table_changes_notify ON ${"schema"}.provider;
  CREATE TRIGGER provider_table_changes_notify
  AFTER INSERT OR UPDATE OR DELETE ON ${"schema"}.provider
  FOR EACH STATEMENT
  EXECUTE FUNCTION ${"schema"}.notify_table();
END $$;

-- llm.notify.provider_group.trigger
DO $$ BEGIN
  DROP TRIGGER IF EXISTS provider_group_table_changes_notify ON ${"schema"}.provider_group;
  CREATE TRIGGER provider_group_table_changes_notify
  AFTER INSERT OR UPDATE OR DELETE ON ${"schema"}.provider_group
  FOR EACH STATEMENT
  EXECUTE FUNCTION ${"schema"}.notify_table();
END $$;
