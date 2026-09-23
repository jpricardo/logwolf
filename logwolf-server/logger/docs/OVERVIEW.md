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
├── cleanup.go  # Background per-project retention cleanup loop, plus the sweep of deleted projects' logs
├── projects.go # Short-lived cache of project ids known to exist, used by LogInfo
├── routes.go   # HTTP route handlers (health check only)
└── rpc.go      # RPCServer type and all RPC method implementations
```

## RPC interface

The RPC server is exposed via Go's standard `net/rpc` package on TCP port 5001.

| Method                | Input               | Output       | Description                                        |
| --------------------- | ------------------- | ------------ | -------------------------------------------------- |
| `RPCServer.LogInfo`   | `RPCLogPayload`     | `string`     | Insert one log entry, unless its project is gone   |
| `RPCServer.GetLogs`   | `QueryParams`       | `[]LogEntry` | Query logs with optional filtering and pagination  |
| `RPCServer.GetLog`    | `RPCLogEntryFilter` | `LogEntry`   | Fetch one entry by id within a project             |
| `RPCServer.DeleteLog` | `RPCLogEntryFilter` | `int64`      | Delete matching log entries; returns count deleted |

API keys live here too, so the Broker needs no database of its own:

| Method                     | Input                   | Output                   | Description                                                      |
| -------------------------- | ----------------------- | ------------------------ | ---------------------------------------------------------------- |
| `RPCServer.ValidateAPIKey` | `RPCValidateAPIKeyArgs` | `RPCValidateAPIKeyReply` | Resolve a plaintext key; unknown or revoked is `Valid` false     |
| `RPCServer.ListAPIKeys`    | `ProjectArgs`           | `[]APIKey`               | A project's keys, newest first                                   |
| `RPCServer.CreateAPIKey`   | `RPCCreateAPIKeyArgs`   | `RPCCreateAPIKeyReply`   | Generate and store a key; the plaintext is returned once         |
| `RPCServer.GetAPIKey`      | `RPCAPIKeyIDArgs`       | `APIKey`                 | Fetch a key by id, for the Broker's membership check             |
| `RPCServer.RevokeAPIKey`   | `RPCRevokeAPIKeyArgs`   | `string`                 | Revoke a key of a project; another project's is `ErrKeyNotFound` |

Replies never carry a key's bcrypt hash. `gob` sends every exported field whatever its `json` tag says, so the logger clears it first.

## HTTP interface

| Method | Path    | Description                   |
| ------ | ------- | ----------------------------- |
| `GET`  | `/ping` | Health check — returns 200 OK |

## Data model

Each log entry stored in MongoDB contains:

| Field        | Type      | Description                   |
| ------------ | --------- | ----------------------------- |
| `_id`        | ObjectID  | MongoDB document ID           |
| `project_id` | string    | Owning project (hex ObjectID) |
| `name`       | string    | Event name                    |
| `data`       | any       | Arbitrary payload             |
| `severity`   | string    | `INFO`, `WARNING`, or `ERROR` |
| `tags`       | []string  | Searchable tags               |
| `duration`   | int64     | Duration in milliseconds      |
| `created_at` | time.Time | Timestamp (drives retention)  |
| `updated_at` | time.Time | Last update timestamp         |

## Retention

Retention is a per-project setting (default 90 days; supported values 30, 60, 90, 180, 365, or 0 for forever). A background loop deletes expired logs project by project every `CLEANUP_INTERVAL`.

Each project gets its own two-minute timeout, so a project with a huge expired set (say, one that just shortened its retention) cannot use up the time of the projects after it in the list. `DeleteExpiredLogs` deletes in batches, so what a project deleted before its timeout stays deleted and the next pass carries on from there.

### Logs of deleted projects

`DeleteProject` does not delete the project's logs itself. There can be millions, and inside the transaction that removes the project they would outlast MongoDB's transaction lifetime limit, so the project could never be deleted. Instead the RPC returns as soon as the project, its keys, settings and members are gone, and hands the project id to the cleanup loop over a small queue. The loop purges its logs in batches (`PurgeProjectLogs`) between passes. If the purge fails, is cut short by shutdown, or the queue is full, the orphan sweep below deletes the rest in a later pass; the project stays deleted either way.

Deleting a project also does not stop every event already addressed to it: the Broker caches API keys for 60 seconds, and events can be waiting in RabbitMQ. Nobody could read those logs, and the retention loop, which walks the existing projects, would never delete them. Two things keep them from piling up:

- `LogInfo` checks that the event's project exists and drops the event with a `project does not exist` error if not. The Listener logs the error and moves on. Project ids seen to exist are cached for a minute, and `DeleteProject` evicts its project from the cache at once, so the check is rarely a database round trip.
- Each cleanup pass also deletes logs whose `project_id` names no project (`DeleteOrphanedLogs`). That catches an event that passed the check just before its project was deleted, or one accepted by another Logger instance whose cache has not expired. It deletes in batches too, with a timeout per batch rather than per pass, so a big deleted project is not cut off every time. Logs with no `project_id`, or an empty one, are left for the startup migration.

Pre-multi-tenancy builds enforced retention with a single global TTL index on `logs.created_at`. That index is dropped on startup — left in place it would keep expiring logs on the old global schedule, overriding whatever each project now has configured.

## Startup migration

Before the RPC server accepts connections, Logger rewrites any `project_members` login that is not lowercase. Membership lookups normalize the login (GitHub logins are case-insensitive), so a row stored as `JDoe` would never match again. Where a project holds one login in several casings, the rows merge into one that keeps the highest role and the oldest join date. Each login merges in its own transaction.

Then it adopts any data written by a pre-multi-tenancy build:

1. Count documents in `logs`, `api_keys`, and `settings` that carry no project ID. If there are none, nothing happens and nothing is logged.
2. Otherwise, find or create a project named `Default` (slug `default`).
3. Stamp every orphaned document with that project's ID.
4. Give each owner login (see below) an owner membership on the project, leaving existing memberships alone.
5. Log a summary line with the project ID and the per-collection counts.

Then, on every start and whatever the orphan count, Logger checks that the `Default` project (if there is one) has an owner. If it has none, every owner login gets an owner membership — a plain member on the list is promoted. A `Default` project that already has an owner is left alone, so someone removed from it in the dashboard is not added back.

The owner logins are `LOGWOLF_ALLOWED_GITHUB_USERS` plus `LOGWOLF_DEFAULT_PROJECT_OWNERS`.

The migration is idempotent — once no orphaned documents remain and `Default` has an owner it is a no-op, so it runs safely on every start. A run that fails partway leaves the rest for the next start and does not stop the service from booting; the failure is logged as `Migration: FAILED`. That includes the owner step: if it fails after the data has moved, the next start finishes it.

During an upgrade, a Broker that validated an API key just before the migration caches that key's project (still empty at the time) for up to 60 seconds, so reads with it can come back empty until the entry expires.

With no owner logins configured the data still migrates, but the Default project has no owner and nobody can see it in the dashboard. Logger logs a warning on every start while this lasts. To recover, set `LOGWOLF_ALLOWED_GITHUB_USERS` or `LOGWOLF_DEFAULT_PROJECT_OWNERS` on Logger and restart it.

Logins allowed only through `LOGWOLF_ALLOWED_GITHUB_ORGS` are not enrolled, because Logger can't see org membership. An org-only deployment names its Default owners in `LOGWOLF_DEFAULT_PROJECT_OWNERS`; those owners then add the rest of the org from the dashboard.

## Graceful shutdown

The HTTP server has a 15-second shutdown timeout. The RPC server closes its TCP listener on signal receipt.

## Environment variables

| Variable                         | Default                 | Description                                                                               |
| -------------------------------- | ----------------------- | ----------------------------------------------------------------------------------------- |
| `MONGO_URL`                      | `mongodb://mongo:27017` | MongoDB connection string                                                                 |
| `LOGGER_RPC_PORT`                | `5001`                  | TCP port for the RPC server                                                               |
| `LOGGER_HTTP_PORT`               | `80`                    | HTTP port for health checks                                                               |
| `CLEANUP_INTERVAL`               | `1h`                    | How often the per-project retention cleanup (and the deleted-project sweep) runs          |
| `LOGWOLF_ALLOWED_GITHUB_USERS`   | —                       | Comma-separated logins made owners of `Default` by the startup migration                  |
| `LOGWOLF_DEFAULT_PROJECT_OWNERS` | —                       | More comma-separated owners of `Default`, for org-only deployments; merged with the above |

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
| Broker   | Calls Logger RPC for reads, deletes and API keys              |
| Listener | Calls Logger RPC to persist events from the queue             |
| MongoDB  | Logger is the sole consumer — no other service touches the DB |

## Note on network isolation

Logger runs on the **internal Docker network** only. It is never reachable from the public internet or from Caddy.
