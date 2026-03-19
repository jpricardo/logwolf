# Contributing to Logwolf

Thanks for your interest in contributing. This document covers how to run the stack locally, how to run the test suite, and the PR process.

## Prerequisites

- Go 1.25+
- Node.js 20+
- Docker and Docker Compose v2
- A GitHub account (for dashboard login during local development)

## Repository layout

```
logwolf/
├── docs/                       # Documentation site (VitePress)
├── logwolf-client/
│   └── js/                     # JS SDK (@logwolf/client-js)
└── logwolf-server/
    ├── broker/                 # HTTP API gateway (Go + chi)
    ├── listener/               # RabbitMQ consumer (Go)
    ├── logger/                 # MongoDB writer + RPC server (Go)
    ├── frontend/               # React Router v7 SSR dashboard (TypeScript)
    ├── toolbox/                # Shared Go module (data models, event types, helpers)
    └── docker-compose.yml      # Full stack orchestration
```

## Running the stack locally

The recommended workflow depends on what you're working on.

### Full stack (simplest)

```bash
cd logwolf-server
cp .env.example .env  # fill in your GitHub OAuth credentials and secrets
docker compose up --build -d
```

Everything runs at `https://localhost`. See the [self-hosting guide](https://docs.logwolf.io/self-hosting) for environment variable details.

### Frontend only

Bring up the infrastructure services and run the frontend dev server with HMR:

```bash
cd logwolf-server
docker compose up -d mongo rabbitmq broker caddy

cd frontend
npm install
npm run dev  # http://localhost:5173
```

Set `API_URL=http://localhost:8080/` in `frontend/.env` if you're running the Broker outside Docker.

### Backend services only

Run infrastructure via Docker, then run Go services individually:

```bash
cd logwolf-server
docker compose up -d mongo rabbitmq

# In separate terminals:
cd logger  && go run ./cmd/api
cd listener && go run ./cmd/api
cd broker   && go run ./cmd/api
```

The Broker listens on port `80` by default. Override with the `BROKER_PORT` env var.

### JS SDK only

```bash
cd logwolf-client/js
npm install
npm test
```

## Running the tests

### Go unit tests

```bash
# Auth middleware tests
cd logwolf-server/broker
go test ./cmd/api/... -v

# Toolbox tests
cd logwolf-server/toolbox
go test ./... -v
```

### Integration tests

Integration tests require Docker. They spin up real MongoDB and RabbitMQ containers via testcontainers-go, launch the full Go service stack as subprocesses, and assert end-to-end behaviour.

```bash
cd logwolf-server/integration
go test -tags integration ./... -v -timeout 5m
```

These tests take 30–60 seconds on first run while container images are pulled.

### JS SDK tests

```bash
cd logwolf-client/js
npm test            # watch mode
npm run coverage    # single run with coverage report
```

### CI

All three test suites run on every push and pull request via GitHub Actions (`.github/workflows/ci.yml`). PRs must pass all checks before merging.

## Code style

**Go** — standard `gofmt` formatting. No linter configuration is enforced yet, but follow the conventions already in the codebase: structured JSON logging, explicit error returns, no package-level globals.

**TypeScript** — Prettier is configured at the repo root (`.prettierrc`). Run it before committing:

```bash
cd logwolf-server/frontend
npx prettier --write .

cd logwolf-client/js
npx prettier --write .
```

## Making changes

### Go services

The Go workspace is defined in `logwolf-server/go.work`. All services import shared code from `toolbox` via `replace` directives. When adding a new dependency to a service, run `go mod tidy` in that service's directory.

### Frontend

The frontend uses React Router v7 with file-based routing. New pages go in `frontend/app/pages/`. Shared UI components go in `frontend/app/components/ui/` — these are shadcn/ui components and should follow the existing pattern.

### JS SDK

The SDK is built with `tsc` and bundled with Rollup. After making changes:

```bash
cd logwolf-client/js
npm run build
```

The public API surface is exported from `lib/index.ts`. Schema changes go in `lib/schema.ts` — all config and event shapes are validated with Zod.

## Pull request process

1. Fork the repository and create a branch from `main`.
2. Make your changes. Add or update tests where relevant.
3. Make sure all tests pass locally before opening a PR.
4. Open a PR against `main` with a clear description of what changed and why.
5. Keep PRs focused — one concern per PR is easier to review than a sprawling change.

There is no formal review SLA. Small, well-scoped PRs get reviewed faster.

## Reporting bugs

Open a GitHub issue. Include:

- What you expected to happen
- What actually happened
- Steps to reproduce
- Logwolf version (or commit SHA)
- Relevant logs (`docker compose logs <service>`)

## License

By contributing, you agree that your contributions will be licensed under the [GNU GPL v3](./LICENSE).
