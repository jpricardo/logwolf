# Self-hosting guide

This guide covers running Logwolf in production on a VPS — real domain, real TLS, persistent data, and a stack that survives reboots.

If you haven't done the quickstart yet, start there first.

## Prerequisites

- A VPS running Linux (Ubuntu 22.04+ recommended)
- Docker Engine and Docker Compose v2 installed
- A domain name with an A record pointing at your server's IP
- Ports 80 and 443 open in your firewall

## Domain setup

Point your domain at the server before starting. Caddy will automatically obtain a Let's Encrypt certificate when it detects a real domain — no manual cert management required.

If you're using a subdomain (e.g. `logs.your-domain.com`), the A record should point to the same IP.

## Update the Caddyfile

The default `Caddyfile` is configured for `localhost`. Replace it with your domain:

```
{
    email your@email.com
}

logs.your-domain.com {
    handle /api/* {
        uri strip_prefix /api
        reverse_proxy broker:80
    }
    handle {
        reverse_proxy frontend:3000
    }
}
```

Caddy will handle TLS automatically. The `email` field is used for Let's Encrypt expiry notifications.

## Environment variables

Create a `.env` file in `logwolf-server/`. The full reference:

| Variable                       | Required    | Description                                                                                         |
| ------------------------------ | ----------- | --------------------------------------------------------------------------------------------------- |
| `GITHUB_CLIENT_ID`             | ✅          | GitHub OAuth app client ID                                                                          |
| `GITHUB_CLIENT_SECRET`         | ✅          | GitHub OAuth app client secret                                                                      |
| `SESSION_SECRET`               | ✅          | Signs session cookies. Minimum 32 random bytes.                                                     |
| `INTERNAL_API_SECRET`          | ✅          | Authenticates Frontend → Broker calls. Minimum 32 random bytes.                                     |
| `LOGWOLF_ALLOWED_GITHUB_USERS` | ✅ (one of) | Comma-separated list of GitHub usernames allowed to access the dashboard                            |
| `LOGWOLF_ALLOWED_GITHUB_ORGS`  | ✅ (one of) | Comma-separated list of GitHub orgs. Any member is allowed.                                         |
| `API_KEY`                      | ✅          | An `lw_`-prefixed API key used by the frontend to instrument itself. Generate one after first boot. |

Generate secrets with:

```bash
openssl rand -hex 32
```

A minimal production `.env`:

```bash
GITHUB_CLIENT_ID=abc123
GITHUB_CLIENT_SECRET=def456
LOGWOLF_ALLOWED_GITHUB_USERS=yourname
SESSION_SECRET=<output of openssl rand -hex 32>
INTERNAL_API_SECRET=<output of openssl rand -hex 32>
API_KEY=lw_<your key from the Keys page>
```

## GitHub OAuth app

Update the **Authorization callback URL** in your GitHub OAuth app to match your domain:

```
https://logs.your-domain.com/auth
```

## Starting the stack

```bash
cd logwolf-server
docker compose up --build -d
```

On first boot, Caddy will request a certificate from Let's Encrypt. This takes a few seconds. Check the logs if the site doesn't come up:

```bash
docker compose logs caddy
```

## Persistence

All data is stored in `db-data/` relative to `logwolf-server/`:

```
db-data/
├── mongo/          # MongoDB data files
├── rabbitmq/       # RabbitMQ state
└── caddy/          # TLS certificates and config
```

Back this directory up regularly. MongoDB stores all your log events here.

A simple backup script:

```bash
#!/bin/bash
tar -czf logwolf-backup-$(date +%Y%m%d).tar.gz logwolf-server/db-data/
```

## Log retention

By default, Logwolf retains logs for 90 days. You can change this from **Settings** in the dashboard without restarting the stack. Available values are 30, 60, 90, 180, 365 days, or forever.

Retention is enforced via a MongoDB TTL index on the `created_at` field. Changing the setting updates the index immediately.

## Automatic restarts

The Broker, Logger, and Frontend services have `restart: always` set in `docker-compose.yml`. They restart automatically after a crash or a server reboot.

To ensure the full stack starts on boot, configure Docker to start on boot:

```bash
sudo systemctl enable docker
```

## Health check

The Broker exposes a public health endpoint — no authentication required:

```bash
curl https://logs.your-domain.com/api/health
```

A healthy response looks like:

```json
{
	"status": "healthy",
	"services": {
		"rabbitmq": { "status": "up" },
		"logger": { "status": "up" }
	}
}
```

Use this endpoint with an uptime monitor (UptimeRobot, Betterstack, etc.) to get alerted if the stack goes down.

## Network security

Logwolf enforces an internal Docker network. The following services are **not** reachable from the public internet:

- MongoDB (`27017`)
- RabbitMQ (`5672`, `15672`)
- Logger RPC (`5001`)
- Listener

Only Caddy is exposed on ports 80 and 443. The Broker and Frontend communicate over the internal Docker network.

## Updating

```bash
cd logwolf-server
git pull
docker compose up --build -d
```

Caddy, MongoDB, and RabbitMQ use pinned image versions in `docker-compose.yml`. Update these deliberately, not automatically.

## Troubleshooting

**The dashboard redirects to GitHub but login fails.**
Check that the Authorization callback URL in your GitHub OAuth app exactly matches `https://your-domain.com/auth`.

**Caddy shows a TLS error.**
Make sure your domain's A record has propagated and ports 80 and 443 are open. Caddy needs port 80 briefly during the ACME challenge even if you only want HTTPS.

**Events aren't appearing in the dashboard.**
Check Listener logs — this is usually a RabbitMQ connectivity issue:

```bash
docker compose logs listener
docker compose logs broker
```

**The stack starts but the dashboard is blank.**
Check Frontend logs:

```bash
docker compose logs frontend
```

A missing `SESSION_SECRET` or `GITHUB_CLIENT_SECRET` will cause silent failures here.
