-- provider.insert
INSERT INTO ${"schema"}.provider (
	"name", provider, url, enabled, credentials, pv, meta
) VALUES (
	@name, @provider, @url, @enabled, @credentials, @pv, @meta
)
RETURNING
	"name",	provider, url, enabled, created_at,	modified_at, COALESCE(meta, '{}'::jsonb) AS meta;

-- provider.select
SELECT
	"name",	provider, url, enabled, created_at,	modified_at, COALESCE(meta, '{}'::jsonb) AS meta
FROM ${"schema"}.provider
WHERE "name"=@name;

-- provider.list
SELECT
	provider."name",	provider.provider, provider.url, provider.enabled, provider.created_at,	provider.modified_at, COALESCE(provider.meta, '{}'::jsonb) AS meta
FROM ${"schema"}.provider AS provider
${where}
${orderby}

-- provider.list_with_credentials
SELECT
	provider."name",	provider.provider, provider.url, provider.enabled, provider.created_at,	provider.modified_at, COALESCE(provider.meta, '{}'::jsonb) AS meta,
	provider.pv, provider.credentials
FROM ${"schema"}.provider AS provider
${where}
${orderby}

-- provider.update
UPDATE ${"schema"}.provider
SET
	${patch},
	modified_at = NOW()
WHERE "name"=@name
RETURNING
	"name",	provider, url, enabled, created_at,	modified_at, COALESCE(meta, '{}'::jsonb) AS meta;

-- provider.delete
DELETE FROM ${"schema"}.provider
WHERE "name"=@name
RETURNING
	"name",	provider, url, enabled, created_at,	modified_at, COALESCE(meta, '{}'::jsonb) AS meta;
