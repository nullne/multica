# Workspaces & Auth

## Problem

Multica is multi-tenant by default. A single deployment must support many independent teams, each with its own issues, agents, daemons, and settings. Identity must work cleanly for two distinct callers — humans (web app) and machines (CLI, daemons, webhooks) — without leaking data across tenants or forcing users to manage parallel login flows.

## Goals

1. **Email-first onboarding** — no password setup; users start with one OTP.
2. **Strict tenant isolation** — every query filters by `workspace_id`; cross-workspace reads require explicit handlers.
3. **Three caller classes**, each with the right token: humans (JWT), CLI/scripts (PAT), daemons (daemon token), inbound webhooks (webhook token).
4. **Workspace settings as a JSONB envelope** so new integrations can land without schema migrations.

## Non-Goals

- SSO (SAML / OIDC) — out of scope until enterprise demand justifies it.
- SCIM provisioning — same.
- Per-project ACLs beyond workspace-level roles.
- Invite flows with onboarding emails (current `CreateMember` is admin-driven, no email send).

## Personas & user stories

- *As a founder spinning up a new team*, I want to sign in with my email, pick a workspace name, and invite my cofounder without learning a permissions model.
- *As an admin*, I want to add a teammate by email and pick their role.
- *As a tech lead*, I want to demote a user from `admin` to `member` without losing their issue history.
- *As an engineer*, I want to mint a personal access token so I can script issue creation from my CLI.
- *As an operations engineer*, I want my daemon's token to authenticate without exposing my account JWT.

## Requirements

### Authentication

| Req | Detail | Priority | Status |
|---|---|---|---|
| Email-OTP login | `POST /auth/firebase` — Firebase-issued ID token, server creates user on first sight | P0 | Shipped |
| Dev-bypass login | `POST /auth/dev` — gated by `DEV_AUTH_BYPASS` env flag; inert in prod | P0 | Shipped |
| Personal access tokens (PAT) | `mul_` prefix; default 90-day TTL; CRUD via `/api/tokens` | P0 | Shipped |
| Daemon tokens | `mdt_` prefix; long-lived; user-scoped owner | P0 | Shipped |
| Webhook tokens | `whk_` prefix; per-webhook; SHA-256 hashed at rest | P0 | Shipped |
| Token revocation | `DELETE /api/tokens/{id}` | P0 | Shipped |
| JWT refresh | None today — tokens are issued at login with a fixed TTL | P1 | Gap |
| OIDC / SSO | — | P2 | Future |

### Workspaces

| Req | Detail | Priority | Status |
|---|---|---|---|
| Create workspace | `POST /api/workspaces` — creator becomes `owner` | P0 | Shipped |
| Slug addressing | Routes like `/w/{slug}/issues/{id}` | P0 | Shipped |
| Update / delete workspace | Admin / owner only | P0 | Shipped |
| Workspace settings JSONB | Arbitrary key/value via `settings` column | P0 | Shipped |
| Provider config | `GET/PATCH /api/workspaces/{id}/providers` — per-provider toggles, default keys | P0 | Shipped |
| Workspace repos | Stored in `settings.repos` JSONB; consumed by daemons to set up worktrees | P0 | Shipped |
| GitHub installation | `GET install-url`, `GET status`, `POST connect`, `DELETE` | P0 | Shipped |
| Telegram group notifications | `GET/PUT/DELETE /api/workspaces/{id}/telegram-notifications` | P0 | Shipped |
| Force-update all daemons | `POST /api/workspaces/{id}/update-all-daemons` (admin) | P0 | Shipped |
| Leave workspace | `POST /api/workspaces/{id}/leave` (member self-service) | P0 | Shipped |

### Members & roles

| Req | Detail | Priority | Status |
|---|---|---|---|
| Roles | `owner | admin | member`; check enforced via middleware | P0 | Shipped |
| Add member | `POST /api/workspaces/{id}/members` — admin/owner only | P0 | Shipped |
| Update / remove member | `PATCH/DELETE /members/{memberId}` — admin/owner only | P0 | Shipped |
| Owner-only ops | `DELETE /api/workspaces/{id}` — owner only | P0 | Shipped |
| Bot users | Synthetic members with `kind='bot'` so webhook-authored comments have a name | P0 | Shipped |
| List bot users | `GET/POST/DELETE /api/workspaces/{id}/bot-users` (admin) | P0 | Shipped |
| User-scoped daemons | Daemons belong to a `user_id`, not a workspace; user picks which workspaces a daemon serves | P0 | Shipped (migration 060) |

### Profile & notifications

| Req | Detail | Priority | Status |
|---|---|---|---|
| Get / update me | `GET/PATCH /api/me` | P0 | Shipped |
| Per-user notification channels | `GET/PUT/DELETE /api/me/notification-channels/telegram` | P0 | Shipped |
| Upload file | `POST /api/upload-file` — returns S3 URL (optionally CloudFront-signed) | P0 | Shipped |

## Data model

```
user
├── id, name, email, avatar_url, kind ('human' | 'bot'), created_at, updated_at
member (user ↔ workspace)
├── workspace_id, user_id, role
workspace
├── id, name, slug, description, settings JSONB
daemon (user-owned)
├── id, user_id, daemon_id, status, ...
daemon_workspace (daemon ↔ workspace assignment)
├── daemon_id, workspace_id, enabled
personal_access_token
├── id, user_id, name, token_hash, token_prefix, expires_at, last_used_at
user_notification_channel
├── user_id, channel_type ('telegram'), channel_id
```

## API surface (summary)

```
POST   /auth/firebase
POST   /auth/dev                              (gated)

GET    /api/me
PATCH  /api/me
GET    /api/me/notification-channels
PUT    /api/me/notification-channels/telegram
DELETE /api/me/notification-channels/telegram
GET    /api/me/daemons
GET    /api/me/daemons/{daemonId}/workspaces
PUT    /api/me/daemons/{daemonId}/workspaces/{workspaceId}

GET    /api/workspaces
POST   /api/workspaces
GET    /api/workspaces/{id}
PUT    /api/workspaces/{id}
DELETE /api/workspaces/{id}                    (owner)
GET    /api/workspaces/{id}/members
POST   /api/workspaces/{id}/members            (admin)
PATCH  /api/workspaces/{id}/members/{memberId} (admin)
DELETE /api/workspaces/{id}/members/{memberId} (admin)
POST   /api/workspaces/{id}/leave
GET    /api/workspaces/{id}/providers          (admin)
PATCH  /api/workspaces/{id}/providers          (admin)
GET    /api/workspaces/{id}/telegram-notifications (admin)
PUT    /api/workspaces/{id}/telegram-notifications (admin)
DELETE /api/workspaces/{id}/telegram-notifications (admin)
GET    /api/workspaces/{id}/github/install-url (admin)
GET    /api/workspaces/{id}/github/status      (admin)
POST   /api/workspaces/{id}/github/connect     (admin)
DELETE /api/workspaces/{id}/github             (admin)
GET    /api/workspaces/{id}/bot-users          (admin)
POST   /api/workspaces/{id}/bot-users          (admin)
DELETE /api/workspaces/{id}/bot-users/{userId} (admin)
POST   /api/workspaces/{id}/update-all-daemons (admin)

GET    /api/tokens
POST   /api/tokens
DELETE /api/tokens/{id}
```

## Acceptance criteria

- Given an email not previously seen, when the user completes Firebase OTP, then a `user` row is created and a JWT is returned.
- Given a `member` role user, when they call any admin-gated handler, then the request returns 403.
- Given a workspace with an owner, when the owner leaves, then the leave is blocked unless another owner exists.
- Given a daemon registered to user A, when user B tries to enable that daemon for their workspace via the daemon-workspaces endpoint, then the request returns 403.

## Open questions

- Should we support **JWT refresh** so the web session can persist across short outages without re-login?
- Should **invite-by-email** generate an email-OTP magic link, or stay admin-managed?
- What happens to **PATs and daemon tokens** when a user is removed from their last workspace? Today they continue to work; should they be revoked?
- Should **bot-user comments** count toward subscriber auto-add the way members do? (Today: no — only `member` and `agent` creators auto-subscribe.)
