# Logwolf Broker

The **Broker** service acts as the public-facing API Gateway for the Logwolf system. It handles all incoming HTTP traffic, serving as the entry point for clients to submit and retrieve logs.

## Overview

This service is designed to decouple log ingestion from processing. It accepts log submissions and immediately offloads them to a message queue for asynchronous processing, ensuring high throughput and low latency for writing clients. For read operations, it acts as a proxy, fetching data from the storage service via RPC.

## Features

- **Log Ingestion (Async)**: Accepts JSON log payloads via HTTP `POST` and pushes them to **RabbitMQ** for processing.
- **Log Retrieval (Sync)**: Handles HTTP `GET` requests and retrieves stored logs by communicating with the **Logger** service via **RPC**.
- **Heartbeat**: Exposes a `/ping` endpoint for health checks.

## Endpoints

| Method | Path    | Description                                                      |
| ------ | ------- | ---------------------------------------------------------------- |
| `POST` | `/logs` | Submit a new log entry. Returns `202 Accepted` immediately.      |
| `GET`  | `/logs` | Retrieve a list of stored logs. Returns `200 OK` with JSON data. |
| `GET`  | `/ping` | Health check.                                                    |

## Dependencies

The Broker requires the following services to function:

- **RabbitMQ**: For publishing log events.
- **Logger Service**: For retrieving historical log data via RPC (TCP).

## Getting Started

The service is containerized and intended to run via Docker Compose.

```bash
# Runs on port 8080 by default in the provided composition
docker compose up -d broker
```
