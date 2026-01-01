# Logwolf Server

## Overview

- **logwolf** is a containerized logging platform composed of a React-based `frontend`, a Go `broker` API, `logger` and `listener` services, and infrastructure services (`mongo`, `rabbitmq`) used for persistence and messaging.
- This repository contains both the server-side services (under logwolf-server) and the client app (under frontend).

---

## 🔧 Quick Start (Full stack with Docker Compose)

1. Build and start everything:
   - `docker-compose up --build -d`
2. Open services:
   - Frontend: http://localhost:3000
   - Broker API: http://localhost:8080
3. Stop:
   - `docker-compose down`

Ports exposed by the compose stack:

- Frontend: 3000
- Broker (API): 8080
- MongoDB: 27017
- RabbitMQ: 5672

> Note: Docker Compose config sets `API_URL` for the `frontend` to `http://broker:80/` when running the stack.

---

## 🚀 Frontend (developer notes)

- Path: frontend
- Dev server:
  - `cd logwolf-server/frontend`
  - `npm install`
  - `npm run dev` (defaults to `http://localhost:5173`)
  - Edit .env to set `API_URL` (defaults to `http://localhost:8080/`)
- Build for production:
  - `npm run build`
  - The build output is under `build/` (contains `client/` and `server/` artifacts)
- Docker:
  - `docker build -t logwolf-frontend ./frontend`
  - `docker run -p 3000:3000 -e API_URL=http://broker:80/ logwolf-frontend`

---

## 🧩 Backend services & how to run locally

- Broker: `logwolf-server/cmd/api` (Go)
  - Run: `go run ./cmd/api` (or build and run the container via Dockerfile)
- Logger & Listener:
  - Similar: `go run ./logger/cmd/api` and `go run ./listener/cmd/api`
- Messaging & DB (from `docker-compose.yml`):
  - Mongo: `MONGO_INITDB_DATABASE=logs`, user `admin`, password `password` from compose defaults
  - RabbitMQ: default settings; volumes persist data under `db-data/rabbitmq/`

---

## ✅ Recommended local workflow

- If developing frontend only:
  - Run backend via `docker-compose up -d broker mongo rabbitmq` and run the frontend locally with `npm run dev`.
- If developing backend only:
  - Run `docker-compose up -d mongo rabbitmq`, then run Go services locally with `go run ...`.
- For full integration tests, run the full compose stack.

---

## 🤝 Contributing

- Fork, create feature branches, and send PRs.
- Keep changes focused; add tests and update docs.

---

## 📜 License

- See LICENSE in repo root.
