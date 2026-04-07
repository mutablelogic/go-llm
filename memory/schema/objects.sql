-- memory.memory
CREATE TABLE IF NOT EXISTS ${"schema"}.memory (
	"session"     UUID NOT NULL REFERENCES ${"llm_schema"}."session" (id) ON DELETE CASCADE,
	"key"         TEXT NOT NULL CHECK ("key" ~ '^[a-zA-Z][a-zA-Z0-9_-]{0,63}$'),
	"value"       TEXT NOT NULL,
	"created_at"  TIMESTAMPTZ NOT NULL DEFAULT now(),
	"modified_at" TIMESTAMPTZ,
	PRIMARY KEY ("session", "key")
);

-- memory.memory_index_search
CREATE INDEX IF NOT EXISTS memory_search_idx
	ON ${"schema"}.memory USING GIN (
		to_tsvector('simple', COALESCE("key", '') || ' ' || COALESCE("value", ''))
	);
