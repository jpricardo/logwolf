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

| Variable                         | Required      | Description                                                                                                                                                                              |
| -------------------------------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GITHUB_CLIENT_ID`               | ✅            | GitHub OAuth app client ID                                                                                                                                                               |
| `GITHUB_CLIENT_SECRET`           | ✅            | GitHub OAuth app client secret                                                                                                                                                           |
| `SESSION_SECRET`                 | ✅            | Signs session cookies. Minimum 32 random bytes.                                                                                                                                          |
| `INTERNAL_API_SECRET`            | ✅            | Authenticates Frontend → Broker calls. Minimum 32 random bytes.                                                                                                                          |
| `LOGWOLF_ALLOWED_GITHUB_USERS`   | ✅ (one of)   | Comma-separated list of GitHub usernames allowed to access the dashboard                                                                                                                 |
| `LOGWOLF_ALLOWED_GITHUB_ORGS`    | ✅ (one of)   | Comma-separated list of GitHub orgs. Any member is allowed.                                                                                                                              |
| `LOGWOLF_DEFAULT_PROJECT_OWNERS` | Upgrades only | GitHub usernames made owners of the `Default` project that holds pre-multi-tenancy data, on top of `LOGWOLF_ALLOWED_GITHUB_USERS`. Needed for org-only deployments. Read on every start. |
| `API_KEY`                        | ✅            | An `lw_`-prefixed API key used by the frontend to instrument itself. Generate one after first boot.                                                                                      |
| `TRUSTED_PROXIES`                | No            | IPs/CIDR ranges whose `X-Forwarded-For` the broker believes, to rate-limit failed API key attempts per client. Compose trusts private ranges; narrow it if you publish the broker port.  |
| `MONGO_USERNAME`                 | ✅            | MongoDB root user. Read by MongoDB only when its data directory is first created; see Updating.                                                                                          |
| `MONGO_PASSWORD`                 | ✅            | Its password. Minimum 32 random bytes on a new install.                                                                                                                                  |
| `RABBITMQ_USERNAME`              | ✅            | RabbitMQ user, created when the node first starts. URL-safe characters only.                                                                                                             |
| `RABBITMQ_PASSWORD`              | ✅            | Its password. URL-safe characters only; `openssl rand -hex 32` is.                                                                                                                       |

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
MONGO_USERNAME=logwolf
MONGO_PASSWORD=<output of openssl rand -hex 32>
RABBITMQ_USERNAME=logwolf
RABBITMQ_PASSWORD=<output of openssl rand -hex 32>
API_KEY=lw_<your key from the Keys page>
```

MongoDB 8.0 needs a CPU with AVX on x86-64 (any 64-bit ARM works). On an old or oddly virtualized VPS without it, `mongod` exits at start with an illegal instruction; check with `grep -m1 -o avx /proc/cpuinfo`.

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

Anything else answers `503` with `"status": "degraded"`. A service is `down` if it cannot be reached, and the logger is `degraded` while its startup tasks (indexes and the data migration) are failing: it keeps serving and retries them in the background, and its `error` says what failed. The logger logs the same error as `Startup: pass N FAILED`.

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

### Upgrading from MongoDB 4.2 and RabbitMQ 3.9

Releases before this one ran `mongo:4.2` and `rabbitmq:3.9`, with the MongoDB credentials fixed at `admin` / `password` and no RabbitMQ credentials at all. They now run MongoDB 8.0 and RabbitMQ 4.3, with credentials from `.env`. A fresh install needs none of this; an existing one needs a few steps, once:

1. **Let queued events drain.** Stop the Broker so nothing new comes in, and wait for the Listener to store what is queued. The new RabbitMQ starts as a fresh node, so anything still queued in the old one stays behind.

   ```bash
   docker compose stop broker
   docker compose logs -f listener   # until it goes quiet
   docker compose down
   ```

2. **Set the new variables in `.env`.** MongoDB reads its credentials only when a data directory is created, so an existing one still has `admin` / `password`; start with those, and change them in step 5. RabbitMQ's are new, so pick them now. `SESSION_SECRET` is now actually used (Compose used to override it with a fixed value), so everyone is signed out once.

   ```bash
   MONGO_USERNAME=admin
   MONGO_PASSWORD=password
   RABBITMQ_USERNAME=logwolf
   RABBITMQ_PASSWORD=<output of openssl rand -hex 32>
   ```

3. **Upgrade the MongoDB data.** MongoDB 8.0 cannot open 4.2 data files directly: they go through 4.4, 5.0, 6.0 and 7.0 first. `scripts/upgrade-mongo.sh` does that on `db-data/mongo`, in throwaway containers without a network. It is safe to run again, but take a backup first.

   ```bash
   cp -a db-data/mongo db-data/mongo.bak
   scripts/upgrade-mongo.sh
   ```

4. **Start the new stack.**

   ```bash
   git pull
   docker compose up --build -d
   ```

5. **Change the MongoDB password** from the old default, then put the new one in `.env` and recreate the containers that use it:

   ```bash
   docker compose exec mongo mongosh -u admin -p password --authenticationDatabase admin \
     --eval 'db.getSiblingDB("admin").changeUserPassword("admin", "<new password>")'
   # set MONGO_PASSWORD=<new password> in .env
   docker compose up -d
   ```

RabbitMQ likewise creates its user only when the node first starts. To change `RABBITMQ_USERNAME` or `RABBITMQ_PASSWORD` later, drain the queue as in step 1, delete `db-data/rabbitmq`, and start again; the Listener declares everything it needs.

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

**After upgrading, the old events are nowhere in the dashboard.**
Logger moves data from before projects existed into a project named `Default`, owned by `LOGWOLF_ALLOWED_GITHUB_USERS` and `LOGWOLF_DEFAULT_PROJECT_OWNERS`. If neither was set, the project has no owner and Logger logs a warning on every start. Set one of them, for org-only deployments `LOGWOLF_DEFAULT_PROJECT_OWNERS`, and restart Logger. The owners then add everyone else from the project's settings page.

```bash
docker compose logs logger | grep Migration
```

**The stack starts but the dashboard is blank.**
Check Frontend logs:

```bash
docker compose logs frontend
```

A missing `GITHUB_CLIENT_SECRET` will cause silent failures here. A missing `SESSION_SECRET`, or any of the MongoDB and RabbitMQ credentials, stops `docker compose` before anything starts, naming the variable.
