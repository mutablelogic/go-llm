-- schema.webhook
-- The webhooks, by their serial numbers, which aren't used again, so a
-- deleted webhook's calls can't be mistaken for another's, with the hash of
-- the token in the URL their service calls, rather than the token
CREATE TABLE IF NOT EXISTS webhook (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  session     TEXT NOT NULL,              -- the session its calls are for
  description TEXT NOT NULL,
  type        TEXT NOT NULL DEFAULT '',   -- its protocol, such as github, or empty, for generic
  secret      TEXT,                       -- what its service signs its calls with, or NULL
  hash        BLOB NOT NULL UNIQUE,       -- the SHA-256 of its token
  created_at  TIMESTAMP NOT NULL
);

-- schema.webhook_session
CREATE INDEX IF NOT EXISTS webhook_session ON webhook (session, id);

-- schema.call
-- The calls of the webhooks, as they were received, by their serial
-- numbers, which increase, so what watches them carries on after the last
-- it had, and which are deleted with their webhook
CREATE TABLE IF NOT EXISTS call (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  webhook     INTEGER NOT NULL REFERENCES webhook (id) ON DELETE CASCADE,
  received_at TIMESTAMP NOT NULL,
  method      TEXT NOT NULL,
  query       TEXT,                       -- its query parameters, as JSON
  headers     TEXT,                       -- the headers which were kept, as JSON
  "values"    TEXT,                       -- its body, as JSON values
  event       TEXT,                       -- what its protocol says happened
  delivery    TEXT,                       -- its delivery's identifier, which a delivery which is sent again has too
  verified    BOOLEAN NOT NULL,
  error       TEXT,                       -- why it was refused, or NULL, when it wasn't
  body        BLOB                        -- its body, as it was sent
);

-- schema.call_webhook
CREATE INDEX IF NOT EXISTS call_webhook ON call (webhook, id);

-- schema.call_delivery
-- A delivery which is sent again is a conflict, of the calls which weren't
-- refused, so a call which was, such as one whose signature was wrong,
-- doesn't stop the delivery being kept, when it's sent again, and is right
CREATE UNIQUE INDEX IF NOT EXISTS call_delivery ON call (webhook, delivery) WHERE delivery IS NOT NULL AND error IS NULL;

-- schema.call_received
CREATE INDEX IF NOT EXISTS call_received ON call (received_at);
