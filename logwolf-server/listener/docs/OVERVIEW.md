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
4. The message is acknowledged once the Logger has stored the event, or once it has been dropped for good (see below).

## Delivery guarantees

Deliveries are acknowledged by hand, and only once settled:

- **Stored** — the logger took the event: ack.
- **Logger unreachable** (dial failure, broken connection, call timeout) — retried with back-off from 1s, doubling to 30s, for as long as it takes. The event holds the queue meanwhile, and the rest wait in RabbitMQ, so events published during a logger outage are stored once it is back.
- **Refused** (`rpc.ServerError`) — an event for a project that does not exist (`data.ErrUnknownProject`) is dropped at once. Any other refusal is retried 5 times, then dropped, so one bad event cannot hold up the queue forever.
- **Malformed message** — dropped. Unknown actions are acked and skipped.
- **Shutdown while retrying** — requeued.

Dropped means rejected without requeue; there is no dead-letter exchange. A message the listener received but had not acknowledged when it stopped goes back to the queue, so delivery is at least once: an event whose store succeeded just before a crash can be stored twice.

The listener keeps one RPC connection to the logger (`loggerClient` in `toolbox/event/logger_client.go`) and dials again only after it breaks. It used to dial for every event and never close the client, which leaked a socket and its goroutines on both ends per event.

## Graceful shutdown

The process listens for `SIGTERM` / `SIGINT`. On shutdown, in-flight message processing is allowed to complete before the connection is closed; an event still waiting on an unreachable logger is requeued instead.

## Environment variables

| Variable          | Default                       | Description                |
| ----------------- | ----------------------------- | -------------------------- |
| `RABBITMQ_URL`    | `amqp://guest:guest@rabbitmq` | RabbitMQ connection string |
| `LOGGER_RPC_ADDR` | `logger:5001`                 | Logger RPC address         |

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
