-- provider.insert
INSERT INTO ${"schema"}.provider (
	"name", provider, url, enabled, "include", "exclude", credentials, pv, meta
) VALUES (
	@name, @provider, @url, @enabled, @include, @exclude, @credentials, @pv, @meta
)
RETURNING
	"name", provider, url, enabled, "include", "exclude", created_at, modified_at,
	COALESCE(meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(pg."group"::text ORDER BY pg."group")
		FROM ${"schema"}.provider_group AS pg
		WHERE pg."provider" = "name"
	), '{}'::text[]) AS groups;

-- provider.select
SELECT
	provider."name", provider.provider, provider.url, provider.enabled, provider."include", provider."exclude", provider.created_at, provider.modified_at,
	COALESCE(provider.meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(pg."group"::text ORDER BY pg."group")
		FROM ${"schema"}.provider_group AS pg
		WHERE pg."provider" = provider."name"
	), '{}'::text[]) AS groups
FROM ${"schema"}.provider AS provider
WHERE "name"=@name;

-- provider.list
SELECT
	provider."name", provider.provider, provider.url, provider.enabled, provider."include", provider."exclude", provider.created_at, provider.modified_at,
	COALESCE(provider.meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(pg."group"::text ORDER BY pg."group")
		FROM ${"schema"}.provider_group AS pg
		WHERE pg."provider" = provider."name"
	), '{}'::text[]) AS groups
FROM ${"schema"}.provider AS provider
${where}
${orderby}

-- provider.list_with_credentials
SELECT
	provider."name", provider.provider, provider.url, provider.enabled, provider."include", provider."exclude", provider.created_at, provider.modified_at,
	COALESCE(provider.meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(pg."group"::text ORDER BY pg."group")
		FROM ${"schema"}.provider_group AS pg
		WHERE pg."provider" = provider."name"
	), '{}'::text[]) AS groups,
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
	"name", provider, url, enabled, "include", "exclude", created_at, modified_at,
	COALESCE(meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(pg."group"::text ORDER BY pg."group")
		FROM ${"schema"}.provider_group AS pg
		WHERE pg."provider" = "name"
	), '{}'::text[]) AS groups;

-- provider.delete
DELETE FROM ${"schema"}.provider
WHERE "name"=@name
RETURNING
	"name", provider, url, enabled, "include", "exclude", created_at, modified_at,
	COALESCE(meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(pg."group"::text ORDER BY pg."group")
		FROM ${"schema"}.provider_group AS pg
		WHERE pg."provider" = "name"
	), '{}'::text[]) AS groups;

-- provider_group.insert
INSERT INTO ${"schema"}.provider_group ("provider", "group")
VALUES (@provider, @group)
ON CONFLICT DO NOTHING
RETURNING "group"::text;

-- provider_group.delete
DELETE FROM ${"schema"}.provider_group
WHERE "provider"=@provider AND "group"=@group
RETURNING "group"::text;

-- provider_group.delete_all
DELETE FROM ${"schema"}.provider_group
WHERE "provider"=@provider
RETURNING "group"::text;

-- provider_group.list
SELECT "group"::text FROM ${"schema"}.provider_group
WHERE "provider"=@provider
ORDER BY "group";
