# Logwolf 🐺

Self-hosted event logging and observability. Your logs stay on your server.

No vendor lock-in. No per-seat pricing. One `docker-compose up` and you're logging.

---

## What it is

Logwolf is a lightweight alternative to Sentry and Datadog for developers who want to own their data. Applications instrument themselves via the JS SDK, which ships structured events to a Go backend pipeline. Events are stored in MongoDB and surfaced through a React dashboard.

- **Capture events** with severity levels, tags, and arbitrary key/value payloads
- **Track durations** automatically — `LogwolfEvent` acts as a stopwatch
- **Sample intelligently** — configurable rates for info, warning, and error events; critical always sends
- **See everything** in a dashboard with metrics, event rate, error rate, and tag breakdowns

## Quick start

```bash
git clone https://github.com/jpricardo/logwolf.git
cd logwolf/logwolf-server
cp .env.example .env   # fill in your GitHub OAuth credentials and secrets
docker compose up --build -d
```

Open `https://localhost`, sign in with GitHub, generate an API key, and start sending events.

Full instructions in the [getting started guide](https://logwolf-docs.vercel.app/getting-started.html).

## JS SDK

```bash
npm install @jpricardo/logwolf-client-js
```

```ts
import Logwolf, { LogwolfEvent } from '@jpricardo/logwolf-client-js';

const logwolf = new Logwolf({
	url: 'https://your-domain.com/api/',
	apiKey: process.env.LOGWOLF_API_KEY,
	sampleRate: 0.5,
	errorSampleRate: 1,
});

const event = new LogwolfEvent({
	name: 'checkout.completed',
	severity: 'info',
	tags: ['payments'],
});

event.set('userId', '123');
event.set('amount', 9900);

await logwolf.capture(event);
```

Full SDK reference at [logwolf-docs.vercel.app/sdk/js](https://logwolf-docs.vercel.app/sdk/js.html).

## Repository layout

```
logwolf/
├── docs/                       # Documentation site (VitePress)
├── logwolf-client/
│   └── js/                     # JS SDK (@jpricardo/logwolf-client-js)
└── logwolf-server/
    ├── broker/                 # HTTP API gateway (Go + chi)
    ├── listener/               # RabbitMQ consumer (Go)
    ├── logger/                 # MongoDB writer + RPC server (Go)
    ├── frontend/               # React Router v7 SSR dashboard (TypeScript)
    ├── toolbox/                # Shared Go module
    └── docker-compose.yml      # Full stack orchestration
```

## Documentation

[logwolf-docs.vercel.app](https://logwolf-docs.vercel.app)

- [Getting started](https://logwolf-docs.vercel.app/getting-started.html) — up and running in 5 minutes
- [Self-hosting guide](https://logwolf-docs.vercel.app/self-hosting.html) — production config, TLS, persistence
- [JS SDK reference](https://logwolf-docs.vercel.app/sdk/js.html) — full API
- [Architecture overview](https://logwolf-docs.vercel.app/architecture.html) — how the pieces fit together

## Stack

| Layer               | Technology                               |
| ------------------- | ---------------------------------------- |
| API gateway         | Go, chi                                  |
| Message queue       | RabbitMQ                                 |
| Database            | MongoDB                                  |
| Dashboard           | React Router v7, Tailwind CSS, shadcn/ui |
| JS SDK              | TypeScript, Zod                          |
| TLS / reverse proxy | Caddy                                    |
| Orchestration       | Docker Compose                           |

## Contributing

Fork, create a feature branch, send a PR. To run the stack locally:

```bash
# Backend services
cd logwolf-server
docker compose up -d mongo rabbitmq
# then run broker, logger, listener individually with go run ./cmd/api

# Frontend
cd logwolf-server/frontend
npm install && npm run dev

# JS SDK
cd logwolf-client/js
npm install && npm test
```

See [logwolf-server/README.md](./logwolf-server/README.md) for the full local development guide.

## License

GNU GPL v3 — see [LICENSE](./LICENSE).
