# Getting started

Logwolf is self-hosted. Your logs never leave your server. This guide gets you from zero to your first event in under five minutes.

## Prerequisites

- A VPS with Docker and Docker Compose installed
- A GitHub account (used to sign in to the dashboard)
- A domain name pointed at your server (optional, but recommended for production)

## 1. Clone the repository

```bash
git clone https://github.com/jpricardo/logwolf.git
cd logwolf/logwolf-server
```

## 2. Create a GitHub OAuth app

Logwolf uses GitHub OAuth to protect the dashboard. You need to create an OAuth app before starting the stack.

Go to **GitHub → Settings → Developer settings → OAuth Apps → New OAuth App** and fill in:

| Field                      | Value                          |
| -------------------------- | ------------------------------ |
| Homepage URL               | `https://your-domain.com`      |
| Authorization callback URL | `https://your-domain.com/auth` |

For local testing, use `https://localhost` and `https://localhost/auth` instead.

Copy the **Client ID** and generate a **Client Secret** — you'll need both in the next step.

## 3. Configure environment variables

Create a `.env` file in `logwolf-server/`:

```bash
# GitHub OAuth
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret

# Restrict dashboard access to specific GitHub users or orgs (comma-separated)
LOGWOLF_ALLOWED_GITHUB_USERS=your_github_username
# LOGWOLF_ALLOWED_GITHUB_ORGS=your-org

# Secrets — generate with: openssl rand -hex 32
SESSION_SECRET=a_long_random_string
INTERNAL_API_SECRET=another_long_random_string
```

## 4. Trust Caddy's local certificate (first run only)

Logwolf serves everything over HTTPS via Caddy. On first run, Caddy issues a self-signed certificate for `localhost`. Trust it once to avoid browser warnings:

```bash
docker compose up -d caddy
docker exec $(docker ps -qf "name=caddy") caddy trust
```

Restart your browser after running this.

## 5. Start the stack

```bash
docker compose up --build -d
```

This starts six services: Caddy, Frontend, Broker, Logger, Listener, MongoDB, and RabbitMQ. Give it 30–60 seconds on first boot for all services to initialise.

## 6. Sign in and generate an API key

Open `https://localhost` (or your domain) in your browser and sign in with GitHub.

Once signed in, go to **API Keys** in the sidebar and click **Generate new key**. Copy the key immediately — it is shown only once and cannot be recovered.

Your key will look like this:

```
lw_A3kB9mXq...
```

## 7. Send your first event

Install the JS SDK:

```bash
npm install @jpricardo/logwolf-client-js
```

Then instrument your application:

```ts
import Logwolf, { LogwolfEvent } from '@jpricardo/logwolf-client-js';

const logwolf = new Logwolf({
	url: 'https://your-domain.com/api/',
	apiKey: process.env.LOGWOLF_API_KEY,
	sampleRate: 1,
	errorSampleRate: 1,
});

const event = new LogwolfEvent({
	name: 'user.signup',
	severity: 'info',
	tags: ['auth'],
});

event.set('userId', '123');
await logwolf.capture(event);
```

Head to the **Dashboard** — your event should appear within a few seconds.

## Stopping the stack

```bash
docker compose down
```

Data is persisted in `db-data/` and survives restarts. To wipe everything and start fresh:

```bash
docker compose down -v
rm -rf db-data/
```

## Next steps

- [Self-hosting guide](./self-hosting.md) — production configuration, persistence, TLS with a real domain
- [JS SDK reference](./sdk/js.md) — full API, sampling, batching, and error capture
