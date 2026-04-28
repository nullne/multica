# Self-Hosting Guide

This guide walks you through deploying Multica on your own infrastructure.

## Architecture Overview

Multica has three components:

| Component | Description | Technology |
|-----------|-------------|------------|
| **Backend** | REST API + WebSocket server | Go (single binary) |
| **Frontend** | Web application | Next.js 16 |
| **Database** | Primary data store | PostgreSQL 17 with pgvector |

Additionally, each user who wants to run AI agents locally installs the **`multica` CLI** and runs the **agent daemon** on their own machine.

## Prerequisites

- Docker and Docker Compose (recommended), or:
  - Go 1.26+ (to build from source)
  - Node.js 20+ and pnpm 10.28+ (to build the frontend)
  - PostgreSQL 17 with the pgvector extension

## Quick Start (Docker Compose)

```bash
git clone https://github.com/nullne/multica.git
cd multica
cp .env.example .env
```

Edit `.env` with your production values (see [Configuration](#configuration) below), then:

```bash
# Start PostgreSQL
docker compose up -d

# Build the backend
make build

# Run database migrations
DATABASE_URL="your-database-url" ./server/bin/migrate up

# Start the backend server
DATABASE_URL="your-database-url" PORT=8080 ./server/bin/server
```

For the frontend:

```bash
pnpm install
pnpm build

# Start the frontend (production mode)
cd apps/web
REMOTE_API_URL=http://localhost:8080 pnpm start
```

## Configuration

All configuration is done via environment variables. Copy `.env.example` as a starting point.

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://multica:multica@localhost:5432/multica?sslmode=disable` |
| `JWT_SECRET` | **Must change from default.** Secret key for signing JWT tokens. Use a long random string. | `openssl rand -hex 32` |
| `FRONTEND_ORIGIN` | URL where the frontend is served (used for CORS) | `https://app.example.com` |

### Firebase Google Auth

Multica signs users in with Firebase Google authentication. Configure both the
server-side project ID and the public Firebase app settings used by the web UI.

| Variable | Description |
|----------|-------------|
| `FIREBASE_PROJECT_ID` | Firebase project ID used to verify Google ID tokens on the server |
| `NEXT_PUBLIC_FIREBASE_API_KEY` | Firebase web app API key |
| `NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN` | Firebase auth domain (for example `your-project.firebaseapp.com`) |
| `NEXT_PUBLIC_FIREBASE_PROJECT_ID` | Firebase project ID exposed to the web app |
| `NEXT_PUBLIC_FIREBASE_APP_ID` | Firebase web app ID |
| `NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID` | Firebase sender ID for the web app |

> **Important — `NEXT_PUBLIC_*` variables are baked in at build time.**
> Next.js inlines `NEXT_PUBLIC_*` values into the client JavaScript bundle
> when the frontend image is built. Setting them at runtime in
> `docker-compose.prod.yml` has no effect on the browser code.
>
> The published `ghcr.io/nullne/multica/frontend` image is built with the
> Multica Cloud Firebase project. To self-host with **your own** Firebase
> project you must rebuild the frontend image locally:
>
> ```bash
> docker build -f apps/web/Dockerfile -t my-multica-frontend:latest \
>   --build-arg NEXT_PUBLIC_FIREBASE_API_KEY=... \
>   --build-arg NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN=... \
>   --build-arg NEXT_PUBLIC_FIREBASE_PROJECT_ID=... \
>   --build-arg NEXT_PUBLIC_FIREBASE_APP_ID=... \
>   --build-arg NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID=... \
>   .
> ```
>
> Then point `docker-compose.prod.yml` at your locally-built image. The
> server-side `FIREBASE_PROJECT_ID` is read at runtime and does not require
> a rebuild.

### File Storage (Optional)

For file uploads and attachments, configure an S3 (or S3-compatible) bucket.
CloudFront-related variables only apply when fronting AWS S3 with CloudFront
signed URLs/cookies; leave them empty for GCS, R2, MinIO, or a public bucket.

| Variable | Description |
|----------|-------------|
| `S3_BUCKET` | Bucket name |
| `S3_REGION` | Region (default: `us-west-2`) |
| `S3_ENDPOINT` | S3-compatible API endpoint. Leave empty for AWS S3. |
| `S3_USE_PATH_STYLE` | `true` / `false`. Defaults to `true` when `S3_ENDPOINT` is set, `false` otherwise. |
| `S3_PUBLIC_URL_BASE` | Optional explicit prefix for public object URLs. Defaults to `CLOUDFRONT_DOMAIN`, then to `<S3_ENDPOINT>/<bucket>`, then to `https://<bucket>`. |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | Credentials. For GCS use HMAC keys; for R2 use the R2 access key pair. |
| `CLOUDFRONT_DOMAIN` | CloudFront distribution domain (AWS only) |
| `CLOUDFRONT_KEY_PAIR_ID` | CloudFront key pair ID for signed URLs (AWS only) |
| `CLOUDFRONT_PRIVATE_KEY` | CloudFront private key in PEM format (AWS only) |
| `COOKIE_DOMAIN` | Domain for CloudFront auth cookies (AWS only) |

**Google Cloud Storage via S3 compatibility**

```bash
S3_BUCKET=my-multica-bucket
S3_REGION=auto
S3_ENDPOINT=https://storage.googleapis.com
S3_PUBLIC_URL_BASE=https://storage.googleapis.com/my-multica-bucket
AWS_ACCESS_KEY_ID=<HMAC access key>
AWS_SECRET_ACCESS_KEY=<HMAC secret>
```

Generate the HMAC key pair from the GCP console
(IAM & Admin → Service Accounts → *service account* → Keys → HMAC keys) and
make sure the bucket grants public read on uploaded objects if you want the
returned URLs to be directly accessible.

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend server port |
| `FRONTEND_PORT` | `3000` | Frontend port |
| `CORS_ALLOWED_ORIGINS` | Value of `FRONTEND_ORIGIN` | Comma-separated list of allowed origins |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

### CLI / Daemon

These are configured on each user's machine, not on the server:

| Variable | Default | Description |
|----------|---------|-------------|
| `MULTICA_SERVER_URL` | `ws://localhost:8080/ws` | WebSocket URL for daemon → server connection |
| `MULTICA_APP_URL` | `http://localhost:3000` | Frontend URL for CLI login flow |
| `MULTICA_DAEMON_POLL_INTERVAL` | `3s` | How often the daemon polls for tasks |
| `MULTICA_DAEMON_HEARTBEAT_INTERVAL` | `15s` | Heartbeat frequency |

## Database Setup

Multica requires PostgreSQL 17 with the pgvector extension.

### Using the Included Docker Compose

```bash
docker compose up -d postgres
```

This starts a `pgvector/pgvector:pg17` container on port 5432 with default credentials (`multica`/`multica`).

### Using Your Own PostgreSQL

Ensure the pgvector extension is available:

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

### Running Migrations

Migrations must be run before starting the server:

```bash
# Using the built binary
./server/bin/migrate up

# Or from source
cd server && go run ./cmd/migrate up
```

## Reverse Proxy

In production, put a reverse proxy in front of both the backend and frontend to handle TLS and routing.

### Caddy (Recommended)

```
app.example.com {
    reverse_proxy localhost:3000
}

api.example.com {
    reverse_proxy localhost:8080
}
```

### Nginx

```nginx
# Frontend
server {
    listen 443 ssl;
    server_name app.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# Backend API
server {
    listen 443 ssl;
    server_name api.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket support
    location /ws {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }
}
```

When using separate domains for frontend and backend, set these environment variables accordingly:

```bash
# Backend
FRONTEND_ORIGIN=https://app.example.com
CORS_ALLOWED_ORIGINS=https://app.example.com

# Frontend
REMOTE_API_URL=https://api.example.com
NEXT_PUBLIC_API_URL=https://api.example.com
NEXT_PUBLIC_WS_URL=wss://api.example.com/ws
```

## Health Check

The backend exposes a health check endpoint:

```
GET /health
→ {"status":"ok"}
```

Use this for load balancer health checks or monitoring.

## Setting Up the Agent Daemon

Each team member who wants to run AI agents locally needs to:

1. **Install the CLI**

   ```bash
   brew tap nullne/tap
   brew install multica-cli
   ```

2. **Install an AI agent CLI** — at least one of:
   - [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (`claude` on PATH)
   - [Codex](https://github.com/openai/codex) (`codex` on PATH)

3. **Authenticate and start**

   ```bash
   # Point CLI to your server
   export MULTICA_APP_URL=https://app.example.com
   export MULTICA_SERVER_URL=wss://api.example.com/ws

   # Login (opens browser)
   multica login

   # Start the daemon
   multica daemon start
   ```

The daemon auto-detects installed agent CLIs and registers itself with the server. When an agent is assigned a task in Multica, the daemon picks it up, creates an isolated workspace, runs the agent, and reports results back.

## Upgrading

1. Pull the latest code or image
2. Run migrations: `./server/bin/migrate up`
3. Restart the backend and frontend

Migrations are forward-only and safe to run on a live database. They are idempotent — running them multiple times has no effect.
