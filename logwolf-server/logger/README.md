# Logwolf Logger

The **Logger** service is the persistent storage engine of the Logwolf system. It is the only service with direct access to the database, serving as the central authority for writing and retrieving log data.

## Overview

This service runs primarily as an **RPC Server**, listening for commands from other services in the mesh. It handles "write" requests from the **Listener** (ingestion) and "read" requests from the **Broker** (retrieval), ensuring all database interactions are centralized and consistent.

## Features

- **RPC Server (TCP)**: Exposes methods for inserting and querying logs via Go's native `net/rpc` package on port `5001`.
- **Database Management**: Manages the connection to **MongoDB** and handles all CRUD operations.
- **HTTP Server**: Runs a lightweight HTTP server on port `80` for health checks (`/ping`).

## RPC API

The service exposes the following RPC methods to internal clients:

- **`RPCServer.LogInfo`**: Accepts a log payload and persists it to the database. Used by the **Listener**.
- **`RPCServer.GetLogs`**: Accepts filter criteria and returns a list of log entries. Used by the **Broker**.

## Dependencies

The Logger requires the following infrastructure:

- **MongoDB**: The backing data store.

## Getting Started

The service is containerized and intended to run via Docker Compose.

```bash
# Build and run the logger in the background
docker compose up -d logger
```
