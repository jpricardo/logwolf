# Architecture overview

Logwolf is a small distributed system. This page explains how the pieces fit together and why they're structured the way they are.

## The pipeline

```
Your app
  │
  │  POST /api/logs   (HTTPS, Bearer token)
  ▼
Caddy  ──────────────────────────────────────────────  Dashboard
  │         reverse proxy                              (React SSR)
  ▼
Broker                                                      │
  │  publishes to RabbitMQ exchange: logs_topic             │
  │  routing keys: log.INFO | log.WARNING | log.ERROR       │
  │                                                         │
  │  GET /api/logs → RPC → Logger ◄────────────────────────┘
  ▼
RabbitMQ
  │
  │  durable queue: logwolf_logs
  ▼
Listener
  │
  │  net/rpc TCP → logger:5001
  ▼
Logger ──────────────────────────────────────────────  MongoDB
       RPCServer.LogInfo (write)
       RPCServer.GetLogs (read)
       RPCServer.DeleteLog (delete)
       RPCServer.GetMetrics (aggregate)
       RPCServer.GetRetention / UpdateRetention
```

## Services

**Caddy** is the only service that faces the internet. It terminates TLS and routes traffic: `/api/*` goes to the Broker, everything else goes to the Frontend. Nothing else is exposed on the host.

**Broker** is the HTTP API gateway, written in Go using the `chi` router. All SDK traffic enters here. It validates API keys, pushes log events to RabbitMQ asynchronously, and proxies read requests to the Logger via RPC. The Broker responds `202 Accepted` to write requests immediately — before the event hits the database. It exposes two write endpoints: `POST /logs` for single events and `POST /logs/batch` for batched delivery (max 1000 events per request).

**RabbitMQ** decouples ingestion from persistence. The Broker publishes events to a topic exchange (`logs_topic`). The Listener consumes from a durable named queue (`logwolf_logs`). If the Listener restarts, in-flight messages are not lost.

**Listener** is a background worker that consumes from RabbitMQ and forwards events to the Logger via Go's `net/rpc` over TCP. It handles one message at a time, synchronously, so a clean shutdown always finishes the current message before stopping.

**Logger** is the only service with direct access to MongoDB. It runs a Go RPC server on port `5001` and handles all reads and writes. The Logger also manages the retention TTL index and runs metric aggregations via a MongoDB `$facet` pipeline.

**Frontend** is a React Router v7 SSR application. It authenticates users via GitHub OAuth, reads data from the Broker using an internal secret header, and instruments itself with the Logwolf SDK (sampling at 50% for normal events, 100% for errors).

**MongoDB** stores all log events in the `logs` collection within the `logs` database. A TTL index on `created_at` enforces the retention policy. MongoDB and RabbitMQ are on an internal Docker network — they are not reachable from the host.

## Write path

When your application calls `logwolf.capture(event)`:

1. The SDK enqueues the event in memory and returns immediately.
2. When the queue reaches `maxBatchSize` or `flushIntervalMs` elapses, the SDK sends a `POST /api/logs/batch` request with an `Authorization: Bearer lw_...` header. Failed sends are retried according to `retryDelaysMs`.
3. Caddy forwards the request to the Broker.
4. The Broker validates the API key — checking an in-memory 60-second cache first, then MongoDB on a miss.
5. The Broker publishes each event in the batch to RabbitMQ and returns `202 Accepted`.
6. The Listener picks up the messages from the durable queue.
7. The Listener calls `RPCServer.LogInfo` on the Logger over TCP.
8. The Logger writes each event to MongoDB.

When `logwolf.create(event)` is used instead, step 1–2 are skipped — the event is sent immediately via `POST /api/logs` and the call awaits the server response.

The HTTP response comes back before the database write completes. This keeps ingestion latency low and protects your application from any slowness in the persistence layer.

## Read path

When the dashboard loads the events list:

1. The Frontend SSR loader calls `GET /api/logs` via the Broker with `X-Internal-Secret`.
2. The Broker dials the Logger on `logger:5001` and calls `RPCServer.GetLogs`.
3. The Logger queries MongoDB with pagination and returns the results.
4. The Broker serialises the result to JSON and returns it to the Frontend.

Every read hits MongoDB directly. There is no read cache.

## Authentication

Logwolf uses two separate authentication mechanisms for two different surfaces.

**API keys** protect the SDK ingestion endpoints (`POST /api/logs`, `GET /api/logs`, `DELETE /api/logs`). Keys use a `lw_` prefix, are stored bcrypt-hashed in MongoDB, and are validated in `requireAPIKey` middleware on the Broker. A 60-second in-memory cache avoids a database hit on every request. Failed attempts are rate-limited per IP: 10 failures within 60 seconds triggers a `429`.

**GitHub OAuth** protects the dashboard. The Frontend handles the OAuth callback, validates the user against a configured allow-list (`LOGWOLF_ALLOWED_GITHUB_USERS` or `LOGWOLF_ALLOWED_GITHUB_ORGS`), and sets a signed HTTP-only session cookie. Dashboard routes are protected at the layout level via `requireAuth`.

Internal Frontend → Broker calls (key management, settings, metrics) use a shared `INTERNAL_API_SECRET` header and never go through the API key path.

## Data model

Each log event is stored as a document in MongoDB with this shape:

```go
type LogEntry struct {
    ID        string    // MongoDB ObjectID as hex string
    Name      string    // Event name, e.g. "checkout.completed"
    Data      string    // JSON payload, stored as a string
    Severity  string    // "info" | "warning" | "error" | "critical"
    Tags      []string
    Duration  int       // milliseconds, measured by the SDK
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

The `Data` field is currently stored as a serialised JSON string rather than a BSON subdocument. This means MongoDB cannot index into the event payload. Migrating to a BSON embedded document is planned and will unlock payload-level querying.

## Network topology

```
Internet
   │
   ▼
[Caddy]  ← ports 80 and 443 only
   │
   ├── public network ──── [Broker]
   │                          │
   │                          └── internal network ──── [Logger]
   │                          │                          [Listener]
   │                          │                          [MongoDB]
   │                          │                          [RabbitMQ]
   │
   └── public network ──── [Frontend]
```

The `internal: true` Docker network flag blocks all external routing. MongoDB and RabbitMQ cannot be reached from outside the host, even if their ports were accidentally exposed.
