# Webhooks & External Integrations

## Problem

Multica's value compounds when external systems can drop work into the agent queue automatically — a Grafana alert opens an incident issue, a GitHub PR comment kicks an agent into review, a deploy failure pages an oncall agent. The system needs a clean separation between **how** an external system reaches us (transport + auth) and **what** we do with the payload (action).

## Goals

1. **Two clean inbound shapes**: a generic token-authenticated webhook (any external system, BYO adapter) and a GitHub App receiver (HMAC-signed, multi-event).
2. **Adapters normalize payloads** into a flat `Data` map keyed by template variables (`{{.title}}`, `{{.labels.app}}`).
3. **Action pipeline** lets one ingest produce multiple side effects in order — today `create_issue` is the only action type but the model supports more.
4. **Per-webhook deduplication** so a paging burst does not create N issues.
5. **Audit trail** for every received event (`webhook_event_log`).

## Non-Goals

- Outbound webhooks (Multica → external). Not in scope today.
- A no-code webhook builder.
- General-purpose Zapier-style automation across arbitrary SaaS endpoints.
- Streaming GitHub events to Slack via Multica.

## Personas & user stories

- *As an SRE*, I want our internal alerting system to POST to Multica and create a triage issue assigned to an agent.
- *As a tech lead*, I want our GitHub PR opens to create a Multica issue with the PR description so an agent can review it.
- *As an admin*, I want to regenerate a webhook token without recreating the webhook.
- *As an admin*, I want to see which incoming payloads were deduplicated.

---

## Part A — Generic Webhooks

The full design is in [`docs/webhook-design.md`](../../webhook-design.md). Summary here.

### Reception flow

```
POST /api/webhooks/{id}    (Authorization: Bearer whk_...)
   │
   ├─ Auth via SHA-256(token) match
   ├─ Lookup webhook (source_type, dedup window, status)
   ├─ Adapter.Parse(payload) → []Event   (one of: standard, oss-alert)
   ├─ Per event:
   │    ├─ Dedup check within window
   │    └─ Run enabled actions in `position` order
   │         └─ "create_issue" → Issue + assign Agent + enqueue task
   └─ 202 Accepted
```

### Adapters

| Adapter | Purpose | Output keys |
|---|---|---|
| `standard` | Multica's native shape — for callers that can pick their schema | `title`, `body`, `priority`, `fields.*` |
| `oss-alert` | Prometheus AlertManager-style ingest | `title`, `body`, `alertname`, `app`, `generator_url`, `labels.*`, `annotations.*` |

Each adapter declares its `Keys[]` (name + description + required flag) and an `Example()` payload — exposed via `GET /api/webhook-adapters` so the webhook-create dialog can render hints.

### Actions

| Type | Config keys | Status |
|---|---|---|
| `create_issue` | `agent_id`, `title_template`, `description_template`, `labels[]`, `dispatch_provider?`, `dispatch_daemon_id?`, `dispatch_daemon_label?` | Shipped |
| `close_issue`, `comment` | — | Planned |

Priority is **not** stored on the action; it is resolved at event time from `payload.priority` if present, otherwise `medium`.

### CRUD endpoints

```
GET    /api/webhooks
POST   /api/webhooks                          (admin)
GET    /api/webhooks/{id}
PUT    /api/webhooks/{id}
DELETE /api/webhooks/{id}
POST   /api/webhooks/{id}/regenerate-token
GET    /api/webhooks/{id}/events              (event log; not yet surfaced in UI)
GET    /api/webhooks/{id}/actions
POST   /api/webhooks/{id}/actions
PUT    /api/webhooks/{id}/actions/{actionId}
DELETE /api/webhooks/{id}/actions/{actionId}
GET    /api/webhook-adapters                  (registry)
GET    /api/webhook-events                    (cross-webhook event log)
```

### Token format

- `whk_` + 40 hex chars
- Stored as `token_hash = SHA-256(token)`, with `token_prefix = first 12 chars` for display
- Bearer auth

### Deduplication

- Per-webhook `dedup_window_seconds` (default 600).
- An event with a `dedup_key` matching a recently processed one within the window is logged as `deduped` and skipped (no action run).

### Issue attribution

Issues created by webhooks have `creator_type = "webhook"` and `creator_id = webhook.id`. Webhook creators are not auto-subscribed (only `member` and `agent` creator types are).

---

## Part B — GitHub App

### Capability summary

| Capability | Status |
|---|---|
| Install GitHub App on org / repos | Shipped |
| Receive `push`, `pull_request`, `issues`, `issue_comment` events | Shipped |
| Per-event-type rule mapping to an agent + workspace | Shipped |
| Templated issue creation from event payload | Shipped |
| HMAC signature verification | Shipped |
| Outbound to GitHub (close PR, post comment) | Not implemented |
| Repo-level filtering | Shipped (migration 049 added `repo` to `github_event_rule`) |
| Inbound webhook unified with generic system | Shipped (migration 050) |

### Setup flow

1. Admin clicks **Connect GitHub** in workspace settings.
2. `GET /api/workspaces/{id}/github/install-url` returns the GitHub App install URL.
3. After install, GitHub redirects to `/github/callback` (web page).
4. Frontend posts `POST /api/workspaces/{id}/github/connect` with the install id.
5. `GET /api/workspaces/{id}/github/status` reports connected installation + repos.

### Event ingest

```
POST /api/github/events
  ├─ Verify HMAC X-Hub-Signature-256
  ├─ Parse webhook event type
  ├─ Lookup workspace by installation id
  ├─ Find matching github_event_rule (event_type, repo)
  ├─ Apply title/description templates
  └─ Create issue + assign agent + enqueue task
```

### Event rule entity

| Field | Description |
|---|---|
| `id`, `workspace_id` | |
| `event_type` | `push | pull_request | issues | issue_comment` |
| `repo` | Repo filter (nullable = all repos) |
| `agent_id` | Which agent to assign |
| `title_template`, `description_template` | Go-template strings with the event payload as context |
| `dispatch_provider`, `dispatch_daemon_id`, `dispatch_daemon_label` | Optional dispatch hints |

### Disconnect

`DELETE /api/workspaces/{id}/github` removes the installation link; rules remain but stop firing.

---

## Part C — Telegram

Covered in [realtime-and-inbox.md](realtime-and-inbox.md). Summary:

- **Per-user channels** — `user_notification_channel(channel_type='telegram')` linked via DM with the workspace bot.
- **Per-workspace group chats** — `workspace.settings.telegram_group`, configured via `/api/workspaces/{id}/telegram-notifications`.
- Inbound is **not** supported (no bot command surface today).

---

## Acceptance criteria (combined)

- Given a generic webhook with a `create_issue` action, when an external system POSTs a valid payload, then within 1s an issue exists and a task is enqueued to the configured agent.
- Given a webhook with `dedup_window_seconds=600`, when two payloads with the same `dedup_key` arrive 5 minutes apart, then only the first creates an issue; the second is logged as `deduped`.
- Given a paused webhook (`status='paused'`), when a payload arrives, then the request returns 200/202 with status `filtered` and no issue is created.
- Given an invalid bearer token, when an external system posts, then the response is 401 and no log row is written.
- Given a GitHub App receiving a `pull_request.opened` event matching a rule, then an issue is created with the templated title and body, assigned to the configured agent.
- Given a GitHub event whose HMAC signature does not match, then the request is rejected with 401 and no issue is created.

## Open questions

- When does **multi-action UI** ship? Backend supports multiple actions per webhook today; the dialog only manages one.
- When is the **event log viewer** wired up? `GET /api/webhooks/{id}/events` exists; the frontend does not surface it yet.
- Should we **support outbound to GitHub** (close PR, post comment)? Document the user need before building.
- Are there **rate limits** we should enforce per webhook (currently unlimited within dedup window)?
- Should the GitHub event rule **fall back to a default agent** when no rule matches a repo, or stay strict (no rule = drop)?
