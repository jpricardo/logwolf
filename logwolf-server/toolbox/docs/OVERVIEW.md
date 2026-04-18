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
│   ├── settings.go  # TTL/retention settings, index management
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

Manages the single-document settings record (currently: retention TTL in days). Also owns the logic for creating/dropping the MongoDB TTL index when retention changes.

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
