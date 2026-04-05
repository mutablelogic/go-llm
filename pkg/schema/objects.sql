-- llm.message_result
DO $$ BEGIN
  CREATE TYPE ${"schema"}.MESSAGE_RESULT AS ENUM ('stop', 'max_tokens', 'blocked', 'tool_call', 'error', 'other', 'max_iterations');
EXCEPTION
  WHEN duplicate_object THEN null;
END $$;

-- llm.job_type
DO $$ BEGIN
  CREATE TYPE ${"schema"}.USAGE_TYPE AS ENUM ('embedding', 'ask', 'chat');
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
  "group"      TEXT NOT NULL REFERENCES ${"auth"}."group" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("provider", "group")
);

-- llm.session
CREATE TABLE IF NOT EXISTS ${"schema"}.session (
    "id"          UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    "parent"      UUID REFERENCES ${"schema"}."session" (id) ON DELETE CASCADE,
    "user"        UUID NOT NULL REFERENCES ${"auth"}."user" (id) ON DELETE CASCADE,
    "name"        TEXT,
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

-- llm.usage
CREATE TABLE IF NOT EXISTS ${"schema"}.usage (
  "id"                  BIGSERIAL PRIMARY KEY,
  "type"                ${"schema"}.USAGE_TYPE NOT NULL,
  "batch"               TEXT,
  "session"             UUID REFERENCES ${"schema"}."session" (id) ON DELETE SET NULL,
  "user"                UUID REFERENCES ${"auth"}."user" (id) ON DELETE SET NULL,
  "provider"            TEXT REFERENCES ${"schema"}.provider ("name") ON DELETE SET NULL,
  "model"               TEXT NOT NULL,
  "input_tokens"        INT,
  "output_tokens"       INT,
  "cache_read_tokens"   INT,
  "cache_write_tokens"  INT,
  "reasoning_tokens"    INT,
  "meta"                JSONB,
  "created_at"          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- llm.usage_index_user
CREATE INDEX IF NOT EXISTS usage_user_created_at_idx
    ON ${"schema"}.usage ("user", "created_at");

-- llm.usage_index_session
CREATE INDEX IF NOT EXISTS usage_session_created_at_idx
    ON ${"schema"}.usage ("session", "created_at");

-- llm.connector
CREATE TABLE IF NOT EXISTS ${"schema"}.connector (
  "url"                 TEXT NOT NULL PRIMARY KEY,
  "namespace"           TEXT NOT NULL,
  "name"                TEXT,
  "title"               TEXT,
  "description"         TEXT,
  "meta"                JSONB,
  "enabled"             BOOLEAN NOT NULL DEFAULT true,
  "created_at"          TIMESTAMPTZ NOT NULL DEFAULT now(),
  "modified_at"         TIMESTAMPTZ,
  "connected_at"        TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS connector_namespace_idx
  ON ${"schema"}.connector ("namespace");

-- llm.connector_group
CREATE TABLE IF NOT EXISTS ${"schema"}.connector_group (
  "connector"   TEXT NOT NULL REFERENCES ${"schema"}.connector ("url") ON DELETE CASCADE,
  "group"       TEXT NOT NULL REFERENCES ${"auth"}."group" ("id") ON DELETE CASCADE,
  PRIMARY KEY ("connector", "group")
);

-- llm.connector_user
CREATE TABLE IF NOT EXISTS ${"schema"}.connector_user (
  "connector"          TEXT NOT NULL REFERENCES ${"schema"}.connector ("url") ON DELETE CASCADE,
  "user"               UUID NOT NULL REFERENCES ${"auth"}."user" (id) ON DELETE CASCADE,
  PRIMARY KEY ("connector", "user")
);

-- llm.credential
CREATE TABLE IF NOT EXISTS ${"schema"}.credential (
  "url"                TEXT NOT NULL,
  "user"               UUID REFERENCES ${"auth"}."user" (id) ON DELETE CASCADE,
  "pv"                 INT NOT NULL DEFAULT 0,
  "credentials"        BYTEA NOT NULL,
  "created_at"         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- llm.credential_index_user
CREATE UNIQUE INDEX IF NOT EXISTS credential_url_user_idx
  ON ${"schema"}.credential ("url", "user")
  WHERE "user" IS NOT NULL;

-- llm.credential_index_global
CREATE UNIQUE INDEX IF NOT EXISTS credential_url_idx
  ON ${"schema"}.credential ("url")
  WHERE "user" IS NULL;

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

-- llm.notify.session.trigger
DO $$ BEGIN
  DROP TRIGGER IF EXISTS session_table_changes_notify ON ${"schema"}.session;
  CREATE TRIGGER session_table_changes_notify
  AFTER INSERT OR UPDATE OR DELETE ON ${"schema"}.session
  FOR EACH STATEMENT
  EXECUTE FUNCTION ${"schema"}.notify_table();
END $$;

-- llm.notify.connector.trigger
DO $$ BEGIN
  DROP TRIGGER IF EXISTS connector_table_changes_notify ON ${"schema"}.connector;
  CREATE TRIGGER connector_table_changes_notify
  AFTER INSERT OR UPDATE OR DELETE ON ${"schema"}.connector
  FOR EACH STATEMENT
  EXECUTE FUNCTION ${"schema"}.notify_table();
END $$;

-- llm.notify.connector_group.trigger
DO $$ BEGIN
  DROP TRIGGER IF EXISTS connector_group_table_changes_notify ON ${"schema"}.connector_group;
  CREATE TRIGGER connector_group_table_changes_notify
  AFTER INSERT OR UPDATE OR DELETE ON ${"schema"}.connector_group
  FOR EACH STATEMENT
  EXECUTE FUNCTION ${"schema"}.notify_table();
END $$;

-- llm.notify.connector_user.trigger
DO $$ BEGIN
  DROP TRIGGER IF EXISTS connector_user_table_changes_notify ON ${"schema"}.connector_user;
  CREATE TRIGGER connector_user_table_changes_notify
  AFTER INSERT OR UPDATE OR DELETE ON ${"schema"}.connector_user
  FOR EACH STATEMENT
  EXECUTE FUNCTION ${"schema"}.notify_table();
END $$;
