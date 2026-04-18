# Listener — Overview

## Purpose

Background worker that consumes log events from RabbitMQ and forwards them to the Logger service via RPC. It is the bridge between the async message queue and durable MongoDB storage.

## Source layout

```
cmd/api/
└── main.go   # Consumer setup, graceful shutdown
```

The Listener is intentionally minimal — all shared logic lives in `logwolf-toolbox`.

## Message flow

```
RabbitMQ (logs_topic exchange)
  └── routing keys: log.INFO, log.WARNING, log.ERROR
      └── Listener consumes
          └── RPC call → Logger:5001 (RPCServer.LogInfo)
              └── MongoDB write
```

1. The Listener binds to the `logs_topic` exchange and subscribes to `log.INFO`, `log.WARNING`, and `log.ERROR` routing keys.
2. Each message is a JSON-encoded `data.RPCLogPayload`.
3. The Listener makes a synchronous RPC call to the Logger service, which writes to MongoDB.
4. Messages are acknowledged after a successful RPC call.

## Graceful shutdown

The process listens for `SIGTERM` / `SIGINT`. On shutdown, in-flight message processing is allowed to complete before the connection is closed.

## Environment variables

| Variable       | Default                       | Description                |
| -------------- | ----------------------------- | -------------------------- |
| `RABBITMQ_URL` | `amqp://guest:guest@rabbitmq` | RabbitMQ connection string |

The Logger RPC address is hard-coded to `logger:5001` (the internal Docker network hostname).

## Dependencies

The Listener has no external dependencies of its own — it uses `logwolf-toolbox` (via the Go workspace) for:

- RabbitMQ connection utilities (`toolbox/rabbitmq`)
- Event consumer logic (`toolbox/event`)
- Shared data models (`toolbox/data`)

## Relationship to other services

| Service  | Relationship                                                         |
| -------- | -------------------------------------------------------------------- |
| RabbitMQ | Listener consumes events from here                                   |
| Logger   | Listener calls Logger RPC to persist events                          |
| Broker   | Broker produces events that Listener consumes (no direct connection) |

## Note on network isolation

Listener runs on the **internal Docker network** only — it is never exposed to the public internet and has no HTTP server of its own.
