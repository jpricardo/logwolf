# Toolbox — Overview

## Purpose

Shared Go library used by Broker, Listener, and Logger. It centralises data models, MongoDB collection helpers, RabbitMQ connection and queue setup, and JSON utilities so that each service stays thin.

Toolbox is a local module in the Go workspace (`logwolf-server/go.work`) and is imported as `logwolf-toolbox/...`. It is never published to an external registry.

## Package layout

```
toolbox/
├── data/
│   ├── models.go    # Models struct, LogEntry, APIKey, Settings types + CRUD methods
│   ├── apikey.go    # APIKey model, key generation, validation
│   ├── settings.go  # Per-project retention settings, index management
│   ├── project.go   # Project and ProjectMember models, membership queries
│   ├── migrate.go   # Startup migration of pre-multi-tenancy data
│   └── log.go       # Log entry type aliases
├── event/
│   ├── event.go     # Exchange + queue declarations
│   ├── emitter.go   # RabbitMQ message publisher
│   └── consumer.go  # RabbitMQ message consumer
├── rabbitmq/
│   └── connect.go   # RabbitMQ connection initialisation
└── json/
    └── helpers.go   # JSON encode/decode utilities
```

## `data` package

### `Models` struct

The central database accessor. Services initialise one `Models` value and pass it around:

```go
models := data.New(mongoClient)
// then use:
models.LogEntry.Insert(entry)
models.LogEntry.AllLogs(queryParams)
models.LogEntry.DeleteOne(filter)
models.APIKey.Insert(key)
models.Settings.Get()
```

### `LogEntry`

Represents a single log record in MongoDB. Key fields: `Name`, `Data`, `Severity`, `Tags`, `Duration`, `CreatedAt`, `UpdatedAt`.

### `APIKey`

Stores API key metadata: key value (hashed), label, created/last-used timestamps. Key generation uses `golang.org/x/crypto` for secure randomness.

### `Settings`

Manages per-project settings documents (currently: retention in days), keyed by `(project_id, key)`.

### Startup migration (`migrate.go`)

Adopts data written before projects existed. Logger calls it on every start; Broker and Listener never do.

| Function                         | Description                                                                             |
| -------------------------------- | --------------------------------------------------------------------------------------- |
| `CountOrphanedDocuments`         | Counts `logs`, `api_keys`, and `settings` documents with no project ID                  |
| `MigrateOrphansToDefaultProject` | Adopts those documents into the `Default` project, creating it and its owners if needed |
| `DropLegacyTTLIndex`             | Removes the global TTL index that predates per-project retention                        |
| `ParseGithubLogins`              | Splits a comma-separated allowlist into logins (trimmed, deduplicated, case preserved)  |

`MigrateOrphansToDefaultProject` returns a nil `*MigrationReport` when there is nothing to adopt, which is what makes repeated runs a no-op.

## `event` package

Declares the RabbitMQ topology used by all services:

- **Exchange**: `logs_topic` (topic type, durable)
- **Named queues**: durable, survive broker restarts
- **Random/exclusive queues**: temporary, used for one-off consumers

`emitter.go` wraps `amqp.Channel.Publish` for structured event publishing.  
`consumer.go` provides `NewConsumer` + `Listen`, the main loop used by Listener.

## `rabbitmq` package

Single `Connect(url string) (*amqp.Connection, error)` function with retry logic for startup ordering (RabbitMQ may not be ready when a service starts).

## `json` package

Lightweight wrappers around `encoding/json` used consistently across services for reading request bodies and writing responses.

## Key dependencies

| Dependency            | Role                  |
| --------------------- | --------------------- |
| `rabbitmq/amqp091-go` | RabbitMQ client       |
| `mongo-driver`        | MongoDB client        |
| `golang.org/x/crypto` | Secure key generation |

## Development

Toolbox has its own unit tests:

```bash
cd logwolf-server/toolbox && go test ./... -v
```

Because Toolbox is a library with no `main` package, it is not run or deployed independently — it is always compiled into the services that depend on it.
