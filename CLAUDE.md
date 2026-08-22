# CLAUDE.md

## Repo overview

Logwolf is a self-hosted logging platform. It is a monorepo with two top-level areas:

- `logwolf-client/js/` — JavaScript SDK (`@logwolf/client-js`) for sending events from applications
- `logwolf-server/` — All backend services, the dashboard frontend, and Docker orchestration

### Backend services (Go)

Managed as a Go workspace (`logwolf-server/go.work`):

| Service    | Path                      | Role                                                                                                       |
| ---------- | ------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `broker`   | `logwolf-server/broker`   | Public HTTP API gateway (chi router). Accepts events, pushes to RabbitMQ; proxies reads to logger via RPC. |
| `listener` | `logwolf-server/listener` | RabbitMQ consumer. Forwards events to logger via RPC.                                                      |
| `logger`   | `logwolf-server/logger`   | Only service with MongoDB access. Dual-server: RPC on port 5001, HTTP health check on port 80.             |
| `toolbox`  | `logwolf-server/toolbox`  | Shared library: data models, RabbitMQ helpers, MongoDB utilities.                                          |

### Frontend (TypeScript)

- `logwolf-server/frontend/` — React Router v7 SSR dashboard (React 19, Tailwind CSS 4, shadcn/ui)
- `logwolf-client/js/` — TypeScript SDK built with Rollup

### Infrastructure

- RabbitMQ for async event ingestion
- MongoDB for persistence
- Caddy as reverse proxy (TLS termination)
- Full stack via `logwolf-server/docker-compose.yml`

## Common commands

### Go services

```bash
# Run a service locally
cd logwolf-server/broker && go run ./cmd/api

# Unit tests (broker + toolbox)
cd logwolf-server/broker && go test ./cmd/api/... -v
cd logwolf-server/toolbox && go test ./... -v

# Integration tests (requires Docker — spins up real MongoDB + RabbitMQ)
cd logwolf-server/integration && go test -tags integration ./... -v -timeout 5m
```

### JS SDK (`logwolf-client/js`)

```bash
npm test          # vitest watch
npm run coverage  # single run with coverage report
npm run build     # tsc + rollup → dist/
npm run lint      # oxlint
npm run format    # oxfmt
npm run typecheck # tsc --noEmit
```

### Frontend (`logwolf-server/frontend`)

```bash
npm run dev       # Vite dev server
npm run build     # react-router build
npm run typecheck # react-router typegen + tsc
npm run lint      # oxlint
```

### Full stack

```bash
# From logwolf-server/
docker compose up
```

## Architecture notes

**Event flow:** Client SDK → Broker (HTTP) → RabbitMQ → Listener → Logger (RPC) → MongoDB

**Networks:** Only Caddy, Broker, and Frontend are on the public network. Logger, Listener, RabbitMQ, and MongoDB are isolated on an internal network.

**API authentication:**

- SDK/API clients: Bearer tokens with `lw_` prefix, validated and cached with TTL + rate limiting (in broker middleware)
- Dashboard: GitHub OAuth 2.0 (user/org allowlist via env vars), iron-session cookies + CSRF tokens on mutations

**Reading vs. writing:** Broker handles writes asynchronously (via RabbitMQ) and reads synchronously (via RPC to logger). Do not add direct DB calls to broker or listener. This holds for both entry points: SDK clients scoped by API key, and the dashboard scoped by project id + membership.

**RabbitMQ topology:** Topic exchange `logs_topic`; routing keys `log.INFO`, `log.WARNING`, `log.ERROR`. Queue declarations live in `toolbox/event/event.go`.

**Data retention:** Retention is per project (default 90 days; supported values are 30/60/90/180/365, or 0 for forever). A cleanup loop in Logger deletes expired logs project by project every `CLEANUP_INTERVAL`. The global TTL index used before multi-tenancy is dropped on startup.

**Startup migration:** Logger adopts pre-multi-tenancy `logs`, `api_keys`, and `settings` (documents with no `project_id`) into a project named `Default` before it serves traffic, making every login in `LOGWOLF_ALLOWED_GITHUB_USERS` an owner. It is idempotent and silent when there is nothing to migrate. See `toolbox/data/migrate.go` and `logger/cmd/api/migrate.go`.

## Service details

### Broker (`logwolf-server/broker`)

Entry point: `cmd/api/main.go`. Key files: `routes.go`, `handlers.go`, `middleware.go`.

- `POST /logs`, `POST /logs/batch` — enqueue events (async, 202)
- `GET /logs`, `DELETE /logs` — proxy to Logger RPC
- Internal routes (`X-Internal-Secret` + `X-User-Login`): `/keys`, `/settings/retention`, `/metrics`, `/projects/...` — including `/projects/{id}/logs`, the dashboard's project-scoped read/write path for events
- `requireAPIKey` middleware caches key lookups; `requireInternalSecret` guards dashboard routes

### Listener (`logwolf-server/listener`)

Entry point: `cmd/api/main.go`. No external dependencies beyond toolbox. Pure consumer — no HTTP server.

### Logger (`logwolf-server/logger`)

Entry point: `cmd/api/main.go`. Key files: `rpc.go`, `routes.go`, `migrate.go`, `cleanup.go`.

RPC methods (Go stdlib `net/rpc`):

- `RPCServer.LogInfo` — insert event
- `RPCServer.GetLogs` — query with pagination/filtering
- `RPCServer.GetLog` — fetch one event by id within a project
- `RPCServer.DeleteLog` — delete by filter, returns count

### Toolbox (`logwolf-server/toolbox`)

Packages: `data` (Models, LogEntry, APIKey, Settings), `event` (emitter + consumer), `rabbitmq` (connection), `json` (helpers).

The `data.Models` struct is the sole database accessor passed between services.

### JS SDK (`logwolf-client/js`)

Key files: `lib/client.ts` (Logwolf class), `lib/schema.ts` (Zod schemas), `lib/event.ts`.

- `capture()` is synchronous; delivery is async and batched
- Configurable `flushInterval`, `maxBatchSize`, `sampleRate`, `errorSampleRate`, `timeout`
- Retry with exponential back-off (3 attempts); FIFO eviction when queue exceeds `maxBatchSize`
- No singleton — callers instantiate their own `Logwolf`

### Frontend (`logwolf-server/frontend`)

Key files: `app/root.tsx`, `app/lib/api.ts` (dashboard API client), `app/lib/auth.server.ts`.

Routes: `/` (public), `/auth`, `/dashboard`, `/events`, `/events/new`, `/events/:id`, `/keys`, `/projects`, `/projects/new`, `/projects/switch`, `/projects/:id/settings`. `/settings` is a redirect to the current project's settings page.

The layout loader keeps `currentProjectID` in the session honest and redirects a user with no projects to `/projects/new`, the only protected page that renders without a current project.

Pages take the current project from the session (`getCurrentProjectID`), never from the URL or a form field, so the redirect back from `/projects/switch` revalidates them into the new project. `/events` included: it goes through the broker's `/projects/:id/logs` routes, not the SDK. The SDK's key belongs to one fixed project, so `lib/logwolf.ts` is now only the dashboard's own error tracking.

Project name, retention, members and deletion all live on `/projects/:id/settings`. Retention is editable by any member; renaming, member changes and deletion are owner-only, enforced in the broker and mirrored in the route so the UI can explain itself.

`lib/api.ts` → calls Broker internal routes via `X-Internal-Secret`, plus `X-User-Login` from the session for the broker's membership checks; project-scoped methods take the project id as an argument. Never calls public SDK routes.

The frontend instruments itself with `@logwolf/client-js` (`lib/logwolf.ts`) for error tracking.

## CI

GitHub Actions (`.github/workflows/ci.yml`) runs on every push to `main` and all PRs:

1. Go unit tests (broker + toolbox)
2. Integration tests
3. JS SDK tests

A separate workflow (`release-js-client.yml`) publishes the JS SDK to npm.

## Environment

Copy `.env.example` to `.env` and fill in GitHub OAuth credentials before running the stack locally. Required vars: `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_ALLOWED_USERS` or `GITHUB_ALLOWED_ORGS`, `SESSION_SECRET`, `API_SECRET`.

Per-service env vars:

| Variable                       | Service          | Default                       | Description                                                            |
| ------------------------------ | ---------------- | ----------------------------- | ---------------------------------------------------------------------- |
| `MONGO_URL`                    | broker, logger   | `mongodb://mongo:27017`       | MongoDB connection                                                     |
| `RABBITMQ_URL`                 | broker, listener | `amqp://guest:guest@rabbitmq` | RabbitMQ connection                                                    |
| `BROKER_PORT`                  | broker           | `80`                          | HTTP listen port                                                       |
| `LOGGER_RPC_PORT`              | logger           | `5001`                        | RPC listen port                                                        |
| `LOGGER_HTTP_PORT`             | logger           | `80`                          | HTTP health check port                                                 |
| `CLEANUP_INTERVAL`             | logger           | `1h`                          | Per-project retention cleanup frequency                                |
| `LOGWOLF_ALLOWED_GITHUB_USERS` | frontend, logger | —                             | Dashboard allowlist; also the owners of the migrated `Default` project |
| `API_URL`                      | frontend         | —                             | Broker base URL                                                        |
| `INTERNAL_API_SECRET`          | frontend         | —                             | Shared secret for internal Broker routes                               |
| `SESSION_SECRET`               | frontend         | —                             | iron-session signing key                                               |

## Detailed docs

Each project has an `OVERVIEW.md` under its `docs/` folder:

- [`logwolf-client/js/docs/OVERVIEW.md`](logwolf-client/js/docs/OVERVIEW.md)
- [`logwolf-server/broker/docs/OVERVIEW.md`](logwolf-server/broker/docs/OVERVIEW.md)
- [`logwolf-server/listener/docs/OVERVIEW.md`](logwolf-server/listener/docs/OVERVIEW.md)
- [`logwolf-server/logger/docs/OVERVIEW.md`](logwolf-server/logger/docs/OVERVIEW.md)
- [`logwolf-server/toolbox/docs/OVERVIEW.md`](logwolf-server/toolbox/docs/OVERVIEW.md)
- [`logwolf-server/frontend/docs/OVERVIEW.md`](logwolf-server/frontend/docs/OVERVIEW.md)
