# Broker — Overview

## Purpose

Public-facing HTTP API gateway. It is the only Logwolf service reachable from the internet (via Caddy). It:

- Authenticates API keys and routes write requests to RabbitMQ
- Proxies read/delete requests to the Logger service via RPC
- Exposes internal admin routes for the dashboard (API key management, settings, metrics)

## Source layout

```
cmd/api/
├── main.go          # Server bootstrap, graceful shutdown
├── routes.go        # Route registration (chi)
├── handlers.go      # Request handlers
├── middleware.go    # Auth middleware (Bearer token, internal secret)
├── rpcerrors.go     # Maps the logger's RPC errors to HTTP statuses
└── helpers.go       # JSON read/write utilities
```

## HTTP routes

### Public routes (Bearer token required)

| Method   | Path          | Scope    | Description                              |
| -------- | ------------- | -------- | ---------------------------------------- |
| `POST`   | `/logs`       | `ingest` | Submit a single log event (async, 202)   |
| `POST`   | `/logs/batch` | `ingest` | Submit up to 1000 events at once         |
| `GET`    | `/logs`       | `read`   | Retrieve events (RPC → Logger → MongoDB) |
| `DELETE` | `/logs`       | `delete` | Delete matching events (RPC → Logger)    |

A key without the route's scope gets 403.

### Internal routes (`X-Internal-Secret` header required)

| Method   | Path                             | Description                                 |
| -------- | -------------------------------- | ------------------------------------------- |
| `GET`    | `/keys`                          | List API keys                               |
| `POST`   | `/keys`                          | Create an API key; `scopes` default: ingest |
| `DELETE` | `/keys/{id}`                     | Revoke an API key                           |
| `GET`    | `/settings/retention`            | Get retention setting                       |
| `PATCH`  | `/settings/retention`            | Update retention TTL                        |
| `GET`    | `/metrics`                       | Usage analytics                             |
| `GET`    | `/projects`                      | Projects the caller belongs to, with `role` |
| `POST`   | `/projects`                      | Create a project (409 if the slug is taken) |
| `GET`    | `/projects/{id}`                 | Get one project                             |
| `PATCH`  | `/projects/{id}`                 | Rename a project                            |
| `DELETE` | `/projects/{id}`                 | Delete a project and everything under it    |
| `GET`    | `/projects/{id}/members`         | List members                                |
| `POST`   | `/projects/{id}/members`         | Add a member                                |
| `PATCH`  | `/projects/{id}/members/{login}` | Change a member's `role` (owner or member)  |
| `DELETE` | `/projects/{id}/members/{login}` | Remove a member                             |
| `GET`    | `/projects/{id}/logs`            | List a project's events (paginated)         |
| `POST`   | `/projects/{id}/logs`            | Submit an event to a project (async, 202)   |
| `GET`    | `/projects/{id}/logs/{logID}`    | Get one event                               |
| `DELETE` | `/projects/{id}/logs/{logID}`    | Delete one event                            |

Internal routes also require `X-User-Login`; project access is checked against
that login on every call. `requireUserLogin` lowercases it first, as memberships
are stored: GitHub logins are case-insensitive.

Renaming or deleting a project and adding, removing or changing the role of a
member are owner-only. A project always keeps one owner: removing or demoting the last one is a 400
(`cannot remove the last owner` / `cannot demote the last owner`). An owner may
demote themselves while another owner remains, which is how a project changes
hands: promote the new owner, then step down.

The `/projects/{id}/logs` routes are the dashboard's way into events. They do the
same work as the public `/logs` routes, but take the project from the path and
check the caller's membership instead of reading it off an API key — the
dashboard authenticates as a user and has no key of its own to scope it. An id
that belongs to another project is a 404, never another project's event.

### Errors from the logger

`net/rpc` turns the logger's errors into plain strings, so `rpcerrors.go` reads
the cause back out of the message in one place (`classifyRPCError`) and
`rpcErrorJSON` maps it to a status, replacing Mongo's wording with a message of
the handler's choosing:

| Cause                                                  | Status | Example                                                                   |
| ------------------------------------------------------ | ------ | ------------------------------------------------------------------------- |
| Unique index violation (`E11000`)                      | 409    | Adding an existing member; renaming to a slug another project already has |
| No document matched, or the id is not a valid ObjectID | 404    | A malformed project id on any project-scoped route                        |
| `data.ErrLastOwner`                                    | 400    | Removing or demoting the last owner                                       |
| Anything else                                          | 500    | The logger or MongoDB failed                                              |

Retention days are checked against `data.ValidRetentionDays` before the logger is
called. A missing `days` is a 400 as well, rather than 0 (keep forever).

### Health

| Method | Path    | Description            |
| ------ | ------- | ---------------------- |
| `GET`  | `/ping` | Health check (no auth) |

## Authentication

Two middleware layers:

- **`requireAPIKey`** — validates the `Authorization: Bearer lw_...` token; keys are cached with TTL + rate limiting to avoid hot-path DB reads.
  - **`requireScope`** — runs after it, per public route, and refuses with 403 a key that lacks the route's scope. Keys can end up in browser bundles, so `POST /keys` gives a key only `ingest` unless the caller asks for `read` or `delete`. A key created before scopes existed has none stored and is read back with all three, so it keeps working. The key cache holds the scopes too, so like revocation, nothing about a key changes for up to 60 seconds.
- **`requireInternalSecret`** — validates the `X-Internal-Secret` header; used exclusively by the dashboard backend.

## Write path

```
Client → POST /logs → requireAPIKey → publish to RabbitMQ → 202 Accepted
```

Events are published to the `logs_topic` exchange with routing key `log.<SEVERITY>`. The broker never writes to MongoDB directly.

## Read path

```
Client    → GET /logs                   → requireAPIKey       → RPC call to Logger:5001 → response
Dashboard → GET /projects/{id}/logs     → membership check    → RPC call to Logger:5001 → response
```

## Environment variables

| Variable       | Default                       | Description                |
| -------------- | ----------------------------- | -------------------------- |
| `MONGO_URL`    | `mongodb://mongo:27017`       | MongoDB connection string  |
| `RABBITMQ_URL` | `amqp://guest:guest@rabbitmq` | RabbitMQ connection string |
| `BROKER_PORT`  | `80`                          | HTTP listen port           |

## Key dependencies

| Dependency            | Role                            |
| --------------------- | ------------------------------- |
| `go-chi/chi`          | HTTP router                     |
| `go-chi/cors`         | CORS middleware                 |
| `rabbitmq/amqp091-go` | RabbitMQ producer               |
| `mongo-driver`        | API key + settings storage      |
| `logwolf-toolbox`     | Shared models and queue helpers |

## Development

```bash
# Run locally
cd logwolf-server/broker && go run ./cmd/api

# Unit tests
cd logwolf-server/broker && go test ./cmd/api/... -v
```

## Relationship to other services

| Service  | Relationship                                        |
| -------- | --------------------------------------------------- |
| RabbitMQ | Broker publishes events here on write               |
| Logger   | Broker calls Logger via RPC on read/delete          |
| Caddy    | Reverse-proxies public traffic to Broker            |
| Frontend | Calls internal routes using the shared `API_SECRET` |
