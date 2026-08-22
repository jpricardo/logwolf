# Logger — Overview

## Purpose

The only Logwolf service with direct MongoDB access. All reads, writes, and deletes on log data go through Logger. It exposes two servers:

- An **RPC server** (port 5001) used by Broker and Listener to perform storage operations.
- A minimal **HTTP server** (port 80) used only for health checks.

## Source layout

```
cmd/api/
├── main.go     # MongoDB setup, indexes, startup migration, dual-server startup, graceful shutdown
├── migrate.go  # Startup migration of pre-multi-tenancy data into the Default project
├── cleanup.go  # Background per-project retention cleanup loop
├── routes.go   # HTTP route handlers (health check only)
└── rpc.go      # RPCServer type and all RPC method implementations
```

## RPC interface

The RPC server is exposed via Go's standard `net/rpc` package on TCP port 5001.

| Method                | Input               | Output       | Description                                        |
| --------------------- | ------------------- | ------------ | -------------------------------------------------- |
| `RPCServer.LogInfo`   | `RPCLogPayload`     | `string`     | Insert a single log entry into MongoDB             |
| `RPCServer.GetLogs`   | `QueryParams`       | `[]LogEntry` | Query logs with optional filtering and pagination  |
| `RPCServer.GetLog`    | `RPCLogEntryFilter` | `LogEntry`   | Fetch one entry by id within a project             |
| `RPCServer.DeleteLog` | `RPCLogEntryFilter` | `int64`      | Delete matching log entries; returns count deleted |

## HTTP interface

| Method | Path    | Description                   |
| ------ | ------- | ----------------------------- |
| `GET`  | `/ping` | Health check — returns 200 OK |

## Data model

Each log entry stored in MongoDB contains:

| Field        | Type      | Description                    |
| ------------ | --------- | ------------------------------ |
| `_id`        | ObjectID  | MongoDB document ID            |
| `project_id` | string    | Owning project (hex ObjectID)  |
| `name`       | string    | Event name                     |
| `data`       | any       | Arbitrary payload              |
| `severity`   | string    | `INFO`, `WARNING`, or `ERROR`  |
| `tags`       | []string  | Searchable tags                |
| `duration`   | int64     | Duration in milliseconds       |
| `created_at` | time.Time | Timestamp (drives retention)   |
| `updated_at` | time.Time | Last update timestamp          |

## Retention

Retention is a per-project setting (default 90 days; supported values 30, 60, 90, 180, 365, or 0 for forever). A background loop deletes expired logs project by project every `CLEANUP_INTERVAL`.

Pre-multi-tenancy builds enforced retention with a single global TTL index on `logs.created_at`. That index is dropped on startup — left in place it would keep expiring logs on the old global schedule, overriding whatever each project now has configured.

## Startup migration

Before the RPC server accepts connections, Logger adopts any data written by a pre-multi-tenancy build:

1. Count documents in `logs`, `api_keys`, and `settings` that carry no project ID. If there are none, nothing happens and nothing is logged.
2. Otherwise, find or create a project named `Default` (slug `default`).
3. Stamp every orphaned document with that project's ID.
4. Give each login in `LOGWOLF_ALLOWED_GITHUB_USERS` an owner membership on the project, leaving existing memberships alone.
5. Log a summary line with the project ID and the per-collection counts.

The migration is idempotent — once no orphaned documents remain it is a no-op, so it runs safely on every start. A run that fails partway leaves the rest for the next start and does not stop the service from booting; the failure is logged as `Migration: FAILED`.

During an upgrade, a Broker that validated an API key just before the migration caches that key's project (still empty at the time) for up to 60 seconds, so reads with it can come back empty until the entry expires.

An empty `LOGWOLF_ALLOWED_GITHUB_USERS` still migrates the data, but the Default project ends up with no owners and stays invisible in the dashboard until a member is added. The migration logs a warning when this happens. Logins allowed only through `LOGWOLF_ALLOWED_GITHUB_ORGS` are not enrolled — the org membership is not visible from Logger.

## Graceful shutdown

The HTTP server has a 15-second shutdown timeout. The RPC server closes its TCP listener on signal receipt.

## Environment variables

| Variable                       | Default                 | Description                                                              |
| ------------------------------ | ----------------------- | ------------------------------------------------------------------------ |
| `MONGO_URL`                    | `mongodb://mongo:27017` | MongoDB connection string                                                |
| `LOGGER_RPC_PORT`              | `5001`                  | TCP port for the RPC server                                              |
| `LOGGER_HTTP_PORT`             | `80`                    | HTTP port for health checks                                              |
| `CLEANUP_INTERVAL`             | `1h`                    | How often the per-project retention cleanup runs                         |
| `LOGWOLF_ALLOWED_GITHUB_USERS` | —                       | Comma-separated logins made owners of `Default` by the startup migration |

## Key dependencies

| Dependency         | Role                                |
| ------------------ | ----------------------------------- |
| `mongo-driver`     | Direct MongoDB access               |
| `logwolf-toolbox`  | Shared data models and utilities    |
| `net/rpc` (stdlib) | RPC server (no external dependency) |

## Development

```bash
# Run locally
cd logwolf-server/logger && go run ./cmd/api

# Unit tests
cd logwolf-server/logger && go test ./... -v
```

The project-scoped RPC methods are covered end to end by the integration suite
(`logwolf-server/integration`), which runs the real Logger against MongoDB.

## Relationship to other services

| Service  | Relationship                                                  |
| -------- | ------------------------------------------------------------- |
| Broker   | Calls Logger RPC for reads and deletes                        |
| Listener | Calls Logger RPC to persist events from the queue             |
| MongoDB  | Logger is the sole consumer — no other service touches the DB |

## Note on network isolation

Logger runs on the **internal Docker network** only. It is never reachable from the public internet or from Caddy.
