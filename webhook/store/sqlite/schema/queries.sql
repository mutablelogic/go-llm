-- webhook.create
INSERT INTO webhook (session, description, type, secret, hash, created_at)
VALUES (@session, @description, @type, nullif(@secret, ''), @hash, @now)
RETURNING id, session, description, type, coalesce(secret, ''), created_at;

-- webhook.get
SELECT id, session, description, type, coalesce(secret, ''), created_at FROM webhook WHERE id = @id;

-- webhook.get_hash
SELECT id, session, description, type, coalesce(secret, ''), created_at FROM webhook WHERE hash = @hash;

-- webhook.list
-- The webhooks, of the session, when there's one, newest first
SELECT id, session, description, type, coalesce(secret, ''), created_at FROM webhook
WHERE (@session = '' OR session = @session)
ORDER BY id DESC

-- webhook.update
-- What's set: its description, and its secret, which is removed when it's
-- empty
UPDATE webhook SET
  description = CASE WHEN @set_description THEN @description ELSE description END,
  secret = CASE WHEN @set_secret THEN nullif(@secret, '') ELSE secret END
WHERE id = @id
RETURNING id, session, description, type, coalesce(secret, ''), created_at;

-- webhook.delete
-- A webhook, and its calls, which cascade
DELETE FROM webhook WHERE id = @id
RETURNING id, session, description, type, coalesce(secret, ''), created_at;

-- call.create
INSERT INTO call (webhook, received_at, method, query, headers, "values", event, delivery, verified, test, error, body)
VALUES (@webhook, @received, @method, @query, @headers, @values, nullif(@event, ''), nullif(@delivery, ''), @verified, @test, nullif(@error, ''), @body)
RETURNING id, webhook, received_at, method, query, headers, "values", coalesce(event, ''), coalesce(delivery, ''), verified, test, coalesce(error, ''), body;

-- call.get
SELECT id, webhook, received_at, method, query, headers, "values", coalesce(event, ''), coalesce(delivery, ''), verified, test, coalesce(error, ''), body
FROM call WHERE id = @id;

-- call.list
-- The calls, of the webhook, or the session's webhooks, when there's one,
-- after the call, and received since the time, which were accepted, or
-- refused, or both, in the order, which is ASC, for the oldest first, or
-- DESC, for the newest
SELECT call.id, call.webhook, call.received_at, call.method, call.query, call.headers, call."values", coalesce(call.event, ''), coalesce(call.delivery, ''), call.verified, call.test, coalesce(call.error, ''), call.body
FROM call JOIN webhook ON webhook.id = call.webhook
WHERE call.id > @after
  AND (@since = '' OR call.received_at >= @since)
  AND (@webhook = 0 OR call.webhook = @webhook)
  AND (@session = '' OR webhook.session = @session)
  AND (@status = '' OR (@status = 'accepted' AND call.error IS NULL) OR (@status = 'refused' AND call.error IS NOT NULL))
ORDER BY call.id ${order}

-- call.delete
DELETE FROM call WHERE id = @id
RETURNING id, webhook, received_at, method, query, headers, "values", coalesce(event, ''), coalesce(delivery, ''), verified, test, coalesce(error, ''), body;

-- call.delete_before
-- The calls received before the time
DELETE FROM call WHERE received_at < @before RETURNING id;
