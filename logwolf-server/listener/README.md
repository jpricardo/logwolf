# Logwolf Listener

The **Listener** service is an asynchronous background worker responsible for consuming log events from the message queue and orchestrating their storage. It bridges the gap between the message broker (RabbitMQ) and the storage engine (Logger).

## Overview

This service is designed to handle high-throughput log processing. Instead of writing directly to the database during a client's HTTP request, the system offloads log entries to a queue. The Listener picks up these messages, processes them, and ensures they are persisted by the Logger service via RPC.

## Features

- **Queue Consumption**: Connects to **RabbitMQ** and binds to specific routing keys (`log.INFO`, `log.WARNING`, `log.ERROR`).
- **Asynchronous Processing**: Decouples the ingestion API from the storage layer, allowing the system to handle spikes in traffic without blocking.
- **RPC Client**: Acts as a client to the **Logger** service, forwarding processed payloads via TCP for persistent storage.

## Data Flow

1. **Consume**: The Listener monitors the `logs_topic` exchange in RabbitMQ.
2. **Process**: When a message arrives, it is unmarshaled from JSON into a Go struct.
3. **Forward**: The service dials the **Logger** service on `logger:5001` and triggers the `RPCServer.LogInfo` method to save the data.

## Dependencies

The Listener requires the following services to be running:

- **RabbitMQ**: Source of log events.
- **Logger Service**: Destination for storage (accessed via RPC).

## Getting Started

The service is containerized and intended to run via Docker Compose.

```bash
# Build and run the listener in the background
docker compose up -d listener
```
