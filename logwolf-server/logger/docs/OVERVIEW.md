# Logger — Overview

## Purpose

The only Logwolf service with direct MongoDB access. All reads, writes, and deletes on log data go through Logger. It exposes two servers:

- An **RPC server** (port 5001) used by Broker and Listener to perform storage operations.
- A minimal **HTTP server** (port 80) used only for health checks.

## Source layout

```
cmd/api/
├── main.go     # MongoDB setup, TTL index, dual-server startup, graceful shutdown
├── routes.go   # HTTP route handlers (health check only)
└── rpc.go      # RPCServer type and all RPC method implementations
```

## RPC interface

The RPC server is exposed via Go's standard `net/rpc` package on TCP port 5001.

| Method                | Input               | Output       | Description                                        |
| --------------------- | ------------------- | ------------ | -------------------------------------------------- |
| `RPCServer.LogInfo`   | `RPCLogPayload`     | `string`     | Insert a single log entry into MongoDB             |
| `RPCServer.GetLogs`   | `QueryParams`       | `[]LogEntry` | Query logs with optional filtering and pagination  |
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
| `name`       | string    | Event name                     |
| `data`       | any       | Arbitrary payload              |
| `severity`   | string    | `INFO`, `WARNING`, or `ERROR`  |
| `tags`       | []string  | Searchable tags                |
| `duration`   | int64     | Duration in milliseconds       |
| `created_at` | time.Time | Timestamp (used for TTL index) |
| `updated_at` | time.Time | Last update timestamp          |

## TTL index

On startup, Logger ensures a TTL index on the `created_at` field of the `logs` collection. The TTL is driven by the retention setting (default 90 days). When the retention setting is updated via Broker's admin API, the index is recreated with the new value.

Supported retention values: 30, 60, 90, 180, 365 days.

## Graceful shutdown

The HTTP server has a 15-second shutdown timeout. The RPC server closes its TCP listener on signal receipt.

## Environment variables

| Variable           | Default                 | Description                 |
| ------------------ | ----------------------- | --------------------------- |
| `MONGO_URL`        | `mongodb://mongo:27017` | MongoDB connection string   |
| `LOGGER_RPC_PORT`  | `5001`                  | TCP port for the RPC server |
| `LOGGER_HTTP_PORT` | `80`                    | HTTP port for health checks |

## Key dependencies

| Dependency         | Role                                |
| ------------------ | ----------------------------------- |
| `mongo-driver`     | Direct MongoDB access               |
| `logwolf-toolbox`  | Shared data models and utilities    |
| `net/rpc` (stdlib) | RPC server (no external dependency) |

## Relationship to other services

| Service  | Relationship                                                  |
| -------- | ------------------------------------------------------------- |
| Broker   | Calls Logger RPC for reads and deletes                        |
| Listener | Calls Logger RPC to persist events from the queue             |
| MongoDB  | Logger is the sole consumer — no other service touches the DB |

## Note on network isolation

Logger runs on the **internal Docker network** only. It is never reachable from the public internet or from Caddy.
