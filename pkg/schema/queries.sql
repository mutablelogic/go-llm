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

-- provider.list_for_user
SELECT
	provider."name", provider.provider, provider.url, provider.enabled, provider."include", provider."exclude", provider.created_at, provider.modified_at,
	COALESCE(provider.meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(pg."group"::text ORDER BY pg."group")
		FROM ${"schema"}.provider_group AS pg
		WHERE pg."provider" = provider."name"
	), '{}'::text[]) AS groups
FROM ${"schema"}.provider AS provider
WHERE (
	NOT EXISTS (
		SELECT 1
		FROM ${"schema"}.provider_group AS provider_group
		WHERE provider_group."provider" = provider."name"
	)
	OR EXISTS (
		SELECT 1
		FROM ${"schema"}.provider_group AS provider_group
		JOIN ${"auth"}.user_group AS user_group ON user_group."group" = provider_group."group"
		WHERE provider_group."provider" = provider."name"
		AND user_group."user" = @user
	)
)
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

-- provider.list_with_credentials_for_user
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
WHERE (
	NOT EXISTS (
		SELECT 1
		FROM ${"schema"}.provider_group AS provider_group
		WHERE provider_group."provider" = provider."name"
	)
	OR EXISTS (
		SELECT 1
		FROM ${"schema"}.provider_group AS provider_group
		JOIN ${"auth"}.user_group AS user_group ON user_group."group" = provider_group."group"
		WHERE provider_group."provider" = provider."name"
		AND user_group."user" = @user
	)
)
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

-- credential.insert
INSERT INTO ${"schema"}.credential (
	url, "user", pv, credentials
) VALUES (
	@url, @user, @pv, @credentials
)
RETURNING
	url, "user", created_at;

-- connector.insert
INSERT INTO ${"schema"}.connector (
	url, namespace, enabled, meta
) VALUES (
	@url, @namespace, @enabled, @meta
)
RETURNING
	url,
	namespace,
	enabled,
	name,
	title,
	description,
	COALESCE(meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(cg."group"::text ORDER BY cg."group")
		FROM ${"schema"}.connector_group AS cg
		WHERE cg."connector" = url
	), '{}'::text[]) AS groups,
	created_at,
	modified_at,
	connected_at;

-- connector.select
SELECT
	connector.url,
	connector.namespace,
	connector.enabled,
	connector.name,
	connector.title,
	connector.description,
	COALESCE(connector.meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(cg."group"::text ORDER BY cg."group")
		FROM ${"schema"}.connector_group AS cg
		WHERE cg."connector" = connector.url
	), '{}'::text[]) AS groups,
	connector.created_at,
	connector.modified_at,
	connector.connected_at
FROM ${"schema"}.connector AS connector
WHERE connector.url = @url;

-- connector.select_for_user
SELECT
	connector.url,
	connector.namespace,
	connector.enabled,
	connector.name,
	connector.title,
	connector.description,
	COALESCE(connector.meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(cg."group"::text ORDER BY cg."group")
		FROM ${"schema"}.connector_group AS cg
		WHERE cg."connector" = connector.url
	), '{}'::text[]) AS groups,
	connector.created_at,
	connector.modified_at,
	connector.connected_at
FROM ${"schema"}.connector AS connector
WHERE connector.url = @url
AND (
	NOT EXISTS (
		SELECT 1
		FROM ${"schema"}.connector_group AS connector_group
		WHERE connector_group."connector" = connector.url
	)
	OR EXISTS (
		SELECT 1
		FROM ${"schema"}.connector_group AS connector_group
		JOIN ${"auth"}.user_group AS user_group ON user_group."group" = connector_group."group"
		WHERE connector_group."connector" = connector.url
		AND user_group."user" = @user
	)
);

-- connector.list
SELECT
	connector.url,
	connector.namespace,
	connector.enabled,
	connector.name,
	connector.title,
	connector.description,
	COALESCE(connector.meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(cg."group"::text ORDER BY cg."group")
		FROM ${"schema"}.connector_group AS cg
		WHERE cg."connector" = connector.url
	), '{}'::text[]) AS groups,
	connector.created_at,
	connector.modified_at,
	connector.connected_at
FROM ${"schema"}.connector AS connector
${where}
${orderby}

-- connector.list_for_user
SELECT
	connector.url,
	connector.namespace,
	connector.enabled,
	connector.name,
	connector.title,
	connector.description,
	COALESCE(connector.meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(cg."group"::text ORDER BY cg."group")
		FROM ${"schema"}.connector_group AS cg
		WHERE cg."connector" = connector.url
	), '{}'::text[]) AS groups,
	connector.created_at,
	connector.modified_at,
	connector.connected_at
FROM ${"schema"}.connector AS connector
WHERE (
	NOT EXISTS (
		SELECT 1
		FROM ${"schema"}.connector_group AS connector_group
		WHERE connector_group."connector" = connector.url
	)
	OR EXISTS (
		SELECT 1
		FROM ${"schema"}.connector_group AS connector_group
		JOIN ${"auth"}.user_group AS user_group ON user_group."group" = connector_group."group"
		WHERE connector_group."connector" = connector.url
		AND user_group."user" = @user
	)
)
${where}
${orderby}

-- connector.update
UPDATE ${"schema"}.connector
SET
	${patch},
	modified_at = NOW()
WHERE url = @url
RETURNING
	url,
	namespace,
	enabled,
	name,
	title,
	description,
	COALESCE(meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(cg."group"::text ORDER BY cg."group")
		FROM ${"schema"}.connector_group AS cg
		WHERE cg."connector" = url
	), '{}'::text[]) AS groups,
	created_at,
	modified_at,
	connected_at;

-- connector.update_state
UPDATE ${"schema"}.connector
SET
	${patch}
WHERE url = @url
RETURNING
	url,
	namespace,
	enabled,
	name,
	title,
	description,
	COALESCE(meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(cg."group"::text ORDER BY cg."group")
		FROM ${"schema"}.connector_group AS cg
		WHERE cg."connector" = url
	), '{}'::text[]) AS groups,
	created_at,
	modified_at,
	connected_at;

-- connector.delete
DELETE FROM ${"schema"}.connector AS connector
WHERE connector.url = @url
RETURNING
	connector.url,
	connector.namespace,
	connector.enabled,
	connector.name,
	connector.title,
	connector.description,
	COALESCE(connector.meta, '{}'::jsonb) AS meta,
	COALESCE((
		SELECT array_agg(cg."group"::text ORDER BY cg."group")
		FROM ${"schema"}.connector_group AS cg
		WHERE cg."connector" = connector.url
	), '{}'::text[]) AS groups,
	connector.created_at,
	connector.modified_at,
	connector.connected_at;

-- connector_group.insert
INSERT INTO ${"schema"}.connector_group ("connector", "group")
VALUES (@connector, @group)
ON CONFLICT DO NOTHING
RETURNING "group"::text;

-- connector_group.delete
DELETE FROM ${"schema"}.connector_group
WHERE "connector"=@connector AND "group"=@group
RETURNING "group"::text;

-- connector_group.delete_all
DELETE FROM ${"schema"}.connector_group
WHERE "connector"=@connector
RETURNING "group"::text;

-- connector_group.list
SELECT "group"::text FROM ${"schema"}.connector_group
WHERE "connector"=@connector
ORDER BY "group";

-- session.insert
INSERT INTO ${"schema"}.session (
	parent, "user", title, meta, tags
) VALUES (
	@parent, @user, @title, @meta, @tags
)
RETURNING
	id,
	parent,
	"user",
	title,
	COALESCE(overhead, 0),
	COALESCE(meta, '{}'::jsonb) AS meta,
	COALESCE(tags, '{}'::text[]) AS tags,
	created_at,
	modified_at;

-- session.list
SELECT
	session.id,
	session.parent,
	session."user",
	session.title,
	COALESCE(session.overhead, 0),
	COALESCE(session.meta, '{}'::jsonb) AS meta,
	COALESCE(session.tags, '{}'::text[]) AS tags,
	session.created_at,
	session.modified_at
FROM ${"schema"}.session AS session
${where}
${orderby}

-- session.select
SELECT
	session.id,
	session.parent,
	session."user",
	session.title,
	COALESCE(session.overhead, 0),
	COALESCE(session.meta, '{}'::jsonb) AS meta,
	COALESCE(session.tags, '{}'::text[]) AS tags,
	session.created_at,
	session.modified_at
FROM ${"schema"}.session AS session
WHERE session.id = @id
${userwhere};

-- session.update
UPDATE ${"schema"}.session
SET
	${patch},
	modified_at = NOW()
WHERE id = @id
${userwhere}
RETURNING
	id,
	parent,
	"user",
	title,
	COALESCE(overhead, 0),
	COALESCE(meta, '{}'::jsonb) AS meta,
	COALESCE(tags, '{}'::text[]) AS tags,
	created_at,
	modified_at;

-- session.delete
DELETE FROM ${"schema"}.session
WHERE id = @id
${userwhere}
RETURNING
	id,
	parent,
	"user",
	title,
	COALESCE(overhead, 0),
	COALESCE(meta, '{}'::jsonb) AS meta,
	COALESCE(tags, '{}'::text[]) AS tags,
	created_at,
	modified_at;

-- usage.insert
INSERT INTO ${"schema"}.usage (
	"type", batch, "session", "user", provider, model,
	input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, reasoning_tokens, meta
) VALUES (
	@type, @batch, @session, @user, @provider, @model,
	@input_tokens, @output_tokens, @cache_read_tokens, @cache_write_tokens, @reasoning_tokens, @meta
)
RETURNING
	id,
	"type"::text,
	batch,
	"session"::text,
	"user"::text,
	provider,
	model,
	COALESCE(input_tokens, 0),
	COALESCE(output_tokens, 0),
	COALESCE(cache_read_tokens, 0),
	COALESCE(cache_write_tokens, 0),
	COALESCE(reasoning_tokens, 0),
	COALESCE(meta, '{}'::jsonb) AS meta,
	created_at;
