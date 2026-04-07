-- memory.insert
INSERT INTO ${"schema"}.memory (
	"session", "key", "value", created_at
) VALUES (
	@session, @key, @value, NOW()
)
RETURNING
	"session",
	"key",
	"value",
	created_at,
	modified_at;

-- memory.select
SELECT
	memory."session",
	memory."key",
	memory."value",
	memory.created_at,
	memory.modified_at
FROM ${"schema"}.memory AS memory
WHERE memory."session" = @session AND memory."key" = @key;

-- memory.list
SELECT
	memory."session",
	memory."key",
	memory."value",
	memory.created_at,
	memory.modified_at
FROM ${"schema"}.memory AS memory
${where}
${orderby}
