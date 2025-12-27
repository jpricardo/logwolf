# Logwolf Server

**Logwolf** is a distributed log aggregation system built with Go. It demonstrates a microservices architecture where ingestion is decoupled from storage, and all database operations are centralized in a single service accessed via RPC.

## Architecture

The system is composed of three main Go services and a shared library, orchestrating data flow through **HTTP**, **AMQP (RabbitMQ)**, and **RPC (TCP)**.

### Services Overview

| Service      | Role           | Description                                                                                                                     |
| ------------ | -------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| **Broker**   | API Gateway    | The public-facing entry point (HTTP). It accepts log submissions and queries. It does not touch the database directly.          |
| **Listener** | Queue Consumer | An asynchronous worker that consumes messages from RabbitMQ and forwards them to the Logger for storage.                        |
| **Logger**   | Storage Engine | The centralized RPC Server that owns the MongoDB connection. It handles both writing (from Listener) and reading (from Broker). |
| **Toolbox**  | Shared Lib     | Contains shared data models, RabbitMQ configuration, and JSON helpers.                                                          |

---

## Data Flow

### 1. Log Ingestion (Asynchronous Write)

This path is optimized for high throughput. The Broker accepts the request immediately, while processing happens in the background.

1. **Client Request**: User sends `POST /logs` to the **Broker**.
2. **Queue Push**: The **Broker** pushes the payload to the `logs_topic` exchange in **RabbitMQ** and responds with `202 Accepted`.
3. **Consumption**: The **Listener** service receives the message from the queue.
4. **RPC Call**: The Listener dials the **Logger** via TCP (`logger:5001`) and calls the `RPCServer.LogInfo` method.
5. **Storage**: The **Logger** inserts the entry into **MongoDB**.

### 2. Log Retrieval (Synchronous Read)

This path allows the Broker to retrieve data without connecting to the database itself.

1. **Client Request**: User sends `GET /logs` to the **Broker**.
2. **RPC Call**:

- The **Broker** dials the **Logger** via TCP (`logger:5001`).
- It executes the `RPCServer.GetLogs` method.

3. **Database Query**: The **Logger** queries **MongoDB** and returns the results to the Broker.
4. **Response**: The Broker formats the data as JSON and sends it to the client.

---

## Technical Stack

- **Language**: Go (Golang) 1.25
- **Communication**:
- **RPC**: Native Go `net/rpc` for inter-service communication (Broker Logger, Listener Logger).
- **AMQP**: `rabbitmq/amqp091-go` for the message queue.
- **REST**: `go-chi/chi` for the HTTP API.
- **Database**: MongoDB (Official `mongo-driver`).

## Project Structure

```text
logwolf-server/
├── broker/     # HTTP API Gateway
├── listener/   # RabbitMQ Worker
├── logger/     # RPC Server & DB Manager
└── toolbox/    # Shared Go Workspace (Models, Event, JSON)

```

## Getting Started

The environment is fully containerized.

### Prerequisites

- Docker

### Run the System

1. **Build and Start**:

```bash
docker compose up --build
```

This will start the following containers:

- `broker`: Mapped to port **8080**.
- `logger`: Internal RPC on port **5001**.
- `listener`: Internal worker.
- `rabbitmq`: Port **5672**.
- `mongo`: Port **27017**.

2. **API Endpoints**:

- **Submit Log**: `POST http://localhost:8080/logs`

```json
{
	"name": "Service A",
	"data": "Something happened",
	"severity": "info",
	"tags": ["something"]
}
```

- **View Logs**: `GET http://localhost:8080/logs`
