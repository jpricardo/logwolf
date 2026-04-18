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
└── helpers.go       # JSON read/write utilities
```

## HTTP routes

### Public routes (Bearer token required)

| Method   | Path          | Description                              |
| -------- | ------------- | ---------------------------------------- |
| `POST`   | `/logs`       | Submit a single log event (async, 202)   |
| `POST`   | `/logs/batch` | Submit up to 1000 events at once         |
| `GET`    | `/logs`       | Retrieve events (RPC → Logger → MongoDB) |
| `DELETE` | `/logs`       | Delete matching events (RPC → Logger)    |

### Internal routes (`X-Internal-Secret` header required)

| Method   | Path                  | Description           |
| -------- | --------------------- | --------------------- |
| `GET`    | `/keys`               | List API keys         |
| `POST`   | `/keys`               | Create an API key     |
| `DELETE` | `/keys/{id}`          | Revoke an API key     |
| `GET`    | `/settings/retention` | Get retention setting |
| `PATCH`  | `/settings/retention` | Update retention TTL  |
| `GET`    | `/metrics`            | Usage analytics       |

### Health

| Method | Path    | Description            |
| ------ | ------- | ---------------------- |
| `GET`  | `/ping` | Health check (no auth) |

## Authentication

Two middleware layers:

- **`requireAPIKey`** — validates the `Authorization: Bearer lw_...` token; keys are cached with TTL + rate limiting to avoid hot-path DB reads.
- **`requireInternalSecret`** — validates the `X-Internal-Secret` header; used exclusively by the dashboard backend.

## Write path

```
Client → POST /logs → requireAPIKey → publish to RabbitMQ → 202 Accepted
```

Events are published to the `logs_topic` exchange with routing key `log.<SEVERITY>`. The broker never writes to MongoDB directly.

## Read path

```
Client → GET /logs → requireAPIKey → RPC call to Logger:5001 → response
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
