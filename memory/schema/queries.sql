-- memory.insert
INSERT INTO ${"schema"}.memory (
	"session", "key", "value", created_at
) VALUES (
	@session, @key, @value, NOW()
)
ON CONFLICT ("session", "key") DO UPDATE
SET
	"value" = EXCLUDED."value",
	modified_at = NOW()
RETURNING
	"session",
	"key",
	"value",
	created_at,
	modified_at;

-- memory.select
WITH RECURSIVE session_tree AS (
	SELECT
		session.id,
		session.parent,
		0 AS depth
	FROM ${"llm_schema"}.session AS session
	WHERE session.id = @session
	UNION ALL
	SELECT
		parent.id,
		parent.parent,
		session_tree.depth + 1
	FROM ${"llm_schema"}.session AS parent
	INNER JOIN session_tree ON session_tree.parent = parent.id
)
SELECT
	memory."session",
	memory."key",
	memory."value",
	memory.created_at,
	memory.modified_at
FROM ${"schema"}.memory AS memory
INNER JOIN session_tree ON session_tree.id = memory."session"
WHERE memory."key" = @key
ORDER BY
	session_tree.depth ASC,
	COALESCE(memory.modified_at, memory.created_at) DESC,
	memory.created_at DESC
LIMIT 1;

-- memory.list_recursive
WITH RECURSIVE session_tree AS (
	SELECT
		session.id,
		session.parent,
		0 AS depth
	FROM ${"llm_schema"}.session AS session
	WHERE session.id = @session
	UNION ALL
	SELECT
		parent.id,
		parent.parent,
		session_tree.depth + 1
	FROM ${"llm_schema"}.session AS parent
	INNER JOIN session_tree ON session_tree.parent = parent.id
), effective_memory AS (
	SELECT DISTINCT ON (memory."key")
		memory."session",
		memory."key",
		memory."value",
		memory.created_at,
		memory.modified_at
	FROM ${"schema"}.memory AS memory
	INNER JOIN session_tree ON session_tree.id = memory."session"
	ORDER BY
		memory."key" ASC,
		session_tree.depth ASC,
		COALESCE(memory.modified_at, memory.created_at) DESC,
		memory.created_at DESC
)
SELECT
	memory."session",
	memory."key",
	memory."value",
	memory.created_at,
	memory.modified_at
FROM effective_memory AS memory
${where}
${orderby}

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
