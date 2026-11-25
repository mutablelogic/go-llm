# Webhook API

## Webhook Operations

A webhook is a URL which a service calls, such as GitHub, when an issue is opened, or Pidgeon, when an email arrives. Each webhook is for a `session`, and has a `description`, and a `type`, which is the protocol of the service which calls it: `generic`, `github`, `aftership` or `pidgeon`. The protocol says how a call is verified, which headers are kept, and what happened, such as `issues.opened`, and which delivery it is, so a delivery which is sent again is only kept once.

A webhook's URL has a random token, which is returned once, when it's created, as its `token`, and its `url`, of the server's public endpoint. Only the token's hash is kept. A webhook can have a `secret`, which its service signs its calls with, which is set when it's created, or later, such as when a service gives it once the URL is registered there. A secret is never returned: `has_secret` says whether there is one.

Each call is kept, as it was received: its method, query, the headers its protocol keeps, its body, as JSON `values`, and as it was sent, what happened, its `delivery`, and whether it was `verified`, or is a `test`. It's pending, when it's accepted, until it's `seen`. A call which is refused, such as one whose signature isn't the webhook's, or whose body is too large, is kept too, with its `error`, so a secret which is wrong can be seen.

### GET /webhook

List the webhooks, newest first, with how many there are. With `session`, only those of the session are listed. Up to 100 are listed at a time, from the `offset`, and at most the `limit`.

### POST /webhook

Create a webhook, for a `session`, with its `description`, `type`, which is `generic` without one, and `secret`, when its service gives one. Returns the webhook, with its `token` and `url`, which aren't returned again. Returns 400 if it has no session, or description, or its type isn't a protocol.

### GET /webhook/{id}

Get the webhook with the serial number. Returns 404 if there's none.

### PATCH /webhook/{id}

Change the webhook's `description`, which can't be empty, or its `secret`, which is removed when it's empty. Returns the webhook. Returns 400 if its description is empty, or 404 if there's none.

### DELETE /webhook/{id}

Delete the webhook, and its calls, so its URL stops working at once. Returns the webhook, as it was. Returns 404 if there's none.

### GET /webhook/call

List the calls, oldest first, after the call with the serial number `after`, with how many there are after it, so what watches them carries on after the last it had, or newest first, with `newest`. With `since`, which is a time, only those received then, or after, to the second, are listed and counted. With `webhook`, or `session`, only those of the webhook, or the session's webhooks, are listed, and with `status`, only those which were `accepted`, or `refused`, or, of those which were accepted, are `pending`, or `seen`. Up to 100 are listed at a time, and at most the `limit`.

### POST /webhook/call

Make a test call to the `webhook`, which its service could make, with a `signature` which is `good`, with the webhook's secret, `bad`, with another, or `none`. Without one, it's `good`, when the webhook has a secret, or else `none`. It's verified, accepted or refused, kept, and published, as any call is, and marked as a `test`, which a service's call can't be. The webhook's URL isn't called. Returns the call, whether it was accepted, or refused, 404 if there's no webhook, or 400 if the signature is `good`, and the webhook has no secret.

### PATCH /webhook/call

Mark the `calls`, which are serial numbers, at most 100, as seen, as they were handled, such as by running an agent with them, so they're no longer pending. One which was seen already, or refused, or deleted, isn't marked. Returns the `count` which weren't seen before. Returns 400 without calls, or with too many.

### GET /webhook/call/{id}

Get the call with the serial number. Returns 404 if there's none.

### DELETE /webhook/call/{id}

Delete the call. Returns the call, as it was. Its delivery is kept again, if it's sent again. Returns 404 if there's none.

## Hook Operations

The URL which a service calls, whose token is a webhook's. It's the only part of the API which a service needs to reach, so it can be the only part which is public.

### POST /hook/{token}

A call of the webhook with the token, which can be a `GET`, `POST` or `PUT`. Returns 202 when it's kept, with its serial number. Returns 200 when it's a delivery which was sent again, which isn't kept again, or a handshake, which a service checks the URL with, such as by echoing a challenge, with its response. Returns 404 if no webhook has the token, 403 if the webhook has a secret, and the call isn't signed with it, and 400 if it isn't a call which the service makes, such as without GitHub's event, its body is too large, or isn't what its content type says. A call which is refused is kept, with why.
