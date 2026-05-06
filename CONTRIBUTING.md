# Contributing Guide

This guide documents the local development workflow for contributors working on the Multica codebase.

It covers:

- first-time setup
- day-to-day development in the main checkout
- isolated worktree development
- the shared PostgreSQL model
- testing and verification
- troubleshooting and destructive reset options

## Development Model

Local development uses one shared PostgreSQL container and one database per checkout.

- the main checkout usually uses `.env` and `POSTGRES_DB=multica`
- each Git worktree uses its own `.env.worktree`
- every checkout connects to the same PostgreSQL host: `localhost:5432`
- isolation happens at the database level, not by starting a separate Docker Compose project
- backend and frontend ports are still unique per worktree

This keeps Docker simple while still isolating schema and data.

## Prerequisites

- Docker (required for all workflows)
- Node.js `v20+` and `pnpm` `v10.28+` (for `make dev-local`)
- Go `v1.26+` (for `make dev-local`)

> **Note:** `make dev` uses Docker exclusively — no local Go or Node toolchain needed.

## Important Rules

- The main checkout should use `.env`.
- A worktree should use `.env.worktree`.
- Do not copy `.env` into a worktree directory.

Why:

- the current command flow prefers `.env` over `.env.worktree`
- if a worktree contains `.env`, it can accidentally point back to the main database

## Environment Files

### Main Checkout

Create `.env` once:

```bash
cp .env.example .env
```

By default, `.env` points to:

```bash
POSTGRES_DB=multica
POSTGRES_PORT=5432
DATABASE_URL=postgres://multica:multica@localhost:5432/multica?sslmode=disable
PORT=8080
FRONTEND_PORT=3000
```

### Worktree

Generate `.env.worktree` from inside the worktree:

```bash
make worktree-env
```

That generates values like:

```bash
POSTGRES_DB=multica_my_feature_702
POSTGRES_PORT=5432
PORT=18782
FRONTEND_PORT=13702
DATABASE_URL=postgres://multica:multica@localhost:5432/multica_my_feature_702?sslmode=disable
```

Notes:

- `POSTGRES_DB` is unique per worktree
- `POSTGRES_PORT` stays fixed at `5432`
- backend and frontend ports are derived from the worktree path hash
- `make worktree-env` refuses to overwrite an existing `.env.worktree`

To regenerate a worktree env file:

```bash
FORCE=1 make worktree-env
```

## First-Time Setup

### Main Checkout

From the main checkout:

```bash
cp .env.example .env
make setup
```

What `make setup` does:

- installs JavaScript dependencies with `pnpm install`
- ensures the shared PostgreSQL container is running
- creates the application database if it does not exist
- runs all migrations against that database

Start the app:

```bash
make dev
```

### Worktree

From the worktree directory:

```bash
make setup-worktree
```

What `make setup-worktree` does:

- generates `.env.worktree` with unique ports
- ensures the shared PostgreSQL container is running
- creates the worktree database if it does not exist
- runs migrations against the worktree database

Start the worktree app:

```bash
make dev-worktree
```

## Recommended Daily Workflow

### Main Checkout

```bash
make dev              # Start dev environment
make test             # Run full test suite
```

### Feature Worktree

```bash
git worktree add ../multica-feature -b feat/my-change main
cd ../multica-feature
make setup-worktree
make dev-worktree
```

After that, day-to-day commands are:

```bash
make dev-worktree
make test
```

## Running Main and Worktree at the Same Time

This is a first-class workflow.

Example:

- main checkout
  - database: `multica`
  - backend: `8080`
  - frontend: `3000`
- worktree checkout
  - database: `multica_my_feature_702`
  - backend: generated worktree port such as `18782`
  - frontend: generated worktree port such as `13702`

Both checkouts use:

- the same PostgreSQL container
- the same PostgreSQL port: `5432`

But they do not share application data, because each uses a different database.

## Command Reference

Run `make` (no arguments) to see all available targets with descriptions.

### Lifecycle

```bash
make setup            # First-time setup: install deps, DB, migrations
make dev              # Start dev via Docker (no local toolchain needed)
make dev-local        # Start dev locally (requires Go + Node)
make up               # Start production-like services via Docker
make down             # Stop all services
make clean            # Stop services and destroy ALL local state
make logs             # Stream production service logs
```

### Testing

```bash
make test             # Run full test suite (typecheck + TS + Go + E2E)
make test-go          # Go tests only
make test-ts          # TypeScript tests only
make test-e2e         # E2E tests only
make check            # TypeScript typecheck only
```

### Build

```bash
make build            # Build all services
make build-backend    # Build Go binaries to server/bin/
make build-frontend   # Build frontend Docker image
```

### Database

```bash
make db-up            # Start shared PostgreSQL container
make db-down          # Stop shared PostgreSQL container
make migrate-up       # Run database migrations
make migrate-down     # Rollback migrations
make sqlc             # Regenerate sqlc code
```

### Worktree

```bash
make worktree-env     # Generate .env.worktree with unique DB/ports
make setup-worktree   # Setup worktree (generate env + setup)
make dev-worktree     # Start dev using worktree config
```

## How Database Creation Works

Database creation is automatic.

The following commands all ensure the target database exists before they continue:

- `make setup`
- `make dev-local`
- `make test`
- `make test-go`
- `make migrate-up`
- `make migrate-down`

That logic lives in `scripts/ensure-postgres.sh`.

## Testing

Run the full test suite:

```bash
make test
```

This runs:

1. TypeScript typecheck
2. TypeScript unit tests
3. Go tests
4. Playwright E2E tests

Notes:

- Go tests create their own fixture data
- E2E tests create their own workspace and issue fixtures
- the test flow starts backend/frontend only if they are not already running

## Local Codex Daemon

Run the local daemon:

```bash
make daemon
```

The daemon authenticates using the CLI's stored token (`multica login`).
It registers runtimes for all watched workspaces from the CLI config.

## Troubleshooting

### Missing Env File

If you see:

```text
Missing env file: .env
```

or:

```text
Missing env file: .env.worktree
```

then create the expected env file first.

Main checkout:

```bash
cp .env.example .env
```

Worktree:

```bash
make worktree-env
```

### Check Which Database a Checkout Uses

Inspect the env file:

```bash
cat .env
cat .env.worktree
```

Look for:

- `POSTGRES_DB`
- `DATABASE_URL`
- `PORT`
- `FRONTEND_PORT`

### List All Local Databases in Shared PostgreSQL

```bash
docker compose exec -T postgres psql -U multica -d postgres -At -c "select datname from pg_database order by datname;"
```

### Worktree Is Accidentally Using the Main Database

Check whether the worktree contains `.env`.

It should not.

The safe worktree setup is:

```bash
make setup-worktree
make dev-worktree
```

### App Stops but PostgreSQL Keeps Running

That is expected. `make down` stops backend/frontend processes but preserves the shared PostgreSQL container.

To stop the shared PostgreSQL container:

```bash
make db-down
```

## Destructive Reset

If you want to stop PostgreSQL and keep your local databases:

```bash
make db-down
```

If you want to wipe all local state (containers, volumes, caches, build artifacts):

```bash
make clean
```

Warning:

- `make clean` removes Docker volumes, which deletes all local databases
- after that you must run `make setup` or `make setup-worktree` again

## Typical Flows

### Stable Main Environment

```bash
cp .env.example .env
make setup
make dev
```

### Feature Worktree

```bash
git worktree add ../multica-feature -b feat/my-change main
cd ../multica-feature
make setup-worktree
make dev-worktree
```

### Return to a Previously Configured Worktree

```bash
cd ../multica-feature
make dev-worktree
```

### Validate Before Pushing

```bash
make test
```

## Publishing a Release

The `CHANGELOG.md` at the repo root is the single source of truth for release notes.  It feeds both the website changelog page (`/changelog`) and the GitHub Release notes generated by GoReleaser.

### Before tagging a release

Add a new section at the top of `CHANGELOG.md` (after the intro, before the previous release):

```markdown
## [0.x.y] - YYYY-MM-DD

### Release Title

- Change one
- Change two
```

Follow the existing format exactly — the website parser and the extraction script both rely on it:

- Version header: `## [x.y.z] - YYYY-MM-DD`
- Section title: `### Title` (one level below the version header)
- Bullet items: `- item text`

### Tagging and shipping

Once `CHANGELOG.md` is updated and merged to `main`:

```bash
git tag v0.x.y
git push origin v0.x.y
```

The `release.yml` workflow automatically extracts the matching section from `CHANGELOG.md` and passes it as the GitHub Release body.  No separate editing of GoReleaser config or i18n files is needed.
