# Logwolf Server

## Overview

- **Logwolf** is a self-hosted, containerized logging platform composed of a React-based `frontend`, a Go `broker` API, `logger` and `listener` services, and infrastructure services (`mongo`, `rabbitmq`) used for persistence and messaging.
- This repository contains both the server-side services (under `logwolf-server`) and the client app (under `frontend`).
- All traffic is served over HTTPS via Caddy, which handles TLS termination and reverse proxying.

---

## 🔧 Quick Start (Full stack with Docker Compose)

### Prerequisites

- Docker Desktop
- A GitHub OAuth App (for dashboard login)

### 1. Create a GitHub OAuth App

Go to **GitHub → Settings → Developer settings → OAuth Apps → New OAuth App** and fill in:

- **Homepage URL:** `https://localhost`
- **Authorization callback URL:** `https://localhost/auth`

Copy the **Client ID** and generate a **Client Secret**.

### 2. Configure environment variables

Create a `.env` file at the repo root (gitignored):

```
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret
LOGWOLF_ALLOWED_GITHUB_USERS=your_github_username
SESSION_SECRET=a_long_random_string
INTERNAL_API_SECRET=another_long_random_string
```

### 3. Trust Caddy's local CA (first run only)

Caddy issues a self-signed certificate for `localhost`. To avoid browser warnings, trust it once:

```bash
docker compose up -d caddy
docker exec $(docker ps -qf "name=caddy") caddy trust
```

Then restart your browser.

### 4. Start the stack

```bash
docker compose up --build -d
```

### 5. Open the dashboard

- **Dashboard:** https://localhost
- **API:** https://localhost/api

### 6. Generate an API key

Sign in via GitHub, navigate to **API Keys**, and generate a key. Copy it immediately — it is shown only once.

### Stop

```bash
docker compose down
```

---

## 🔑 Authentication

Logwolf uses two separate auth mechanisms:

| Surface   | Mechanism                                                               |
| --------- | ----------------------------------------------------------------------- |
| SDK / API | Static API keys (`lw_` prefix, passed as `Authorization: Bearer <key>`) |
| Dashboard | GitHub OAuth 2.0, signed HTTP-only session cookie                       |

Access to the dashboard is restricted to GitHub users or organizations listed in `LOGWOLF_ALLOWED_GITHUB_USERS` or `LOGWOLF_ALLOWED_GITHUB_ORGS`.

> **Security note:** The Logger RPC port (`5001`) and MongoDB are not exposed to the host. All internal service communication happens over an isolated Docker network. Never expose these ports externally.

---

## 📦 JS SDK

Install the SDK:

```bash
npm install @jpricardo/logwolf-client-js
```

Initialize with your API key:

```ts
import Logwolf from '@jpricardo/logwolf-client-js';

const logwolf = new Logwolf({
	url: 'https://your-logwolf-instance/api/',
	apiKey: process.env.LOGWOLF_API_KEY,
	sampleRate: 0.5,
	errorSampleRate: 1,
});
```

---

## 🚀 Frontend (developer notes)

- Path: `frontend`
- Dev server:
  - `cd logwolf-server/frontend`
  - `npm install`
  - `npm run dev` (defaults to `http://localhost:5173`)
  - Set `API_URL` in `.env` (defaults to `http://localhost:8080/`)
- Build for production:
  - `npm run build`
  - Output is under `build/` (`client/` and `server/` artifacts)

---

## 🧩 Backend services & how to run locally

- **Broker:** `logwolf-server/broker/cmd/api` — `go run ./cmd/api`
- **Logger:** `logwolf-server/logger/cmd/api` — `go run ./cmd/api`
- **Listener:** `logwolf-server/listener/cmd/api` — `go run ./cmd/api`
- **Mongo & RabbitMQ:** run via Docker Compose — `docker compose up -d mongo rabbitmq`

---

## ✅ Recommended local workflow

- **Frontend only:** `docker compose up -d broker mongo rabbitmq caddy`, then `npm run dev` in `frontend/`.
- **Backend only:** `docker compose up -d mongo rabbitmq`, then run Go services locally.
- **Full stack:** `docker compose up --build -d`

---

## 🤝 Contributing

Fork, create feature branches, and send PRs. Keep changes focused; add tests and update docs.

---

## 📜 License

See LICENSE in repo root.
