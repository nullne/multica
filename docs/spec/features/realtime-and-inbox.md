# Real-time, Inbox & Activity Log

## Problem

A board where agents are working in parallel is only useful if humans see status changes immediately. The system has to (1) broadcast every mutation in real-time, (2) route notifications to the right people via the right channel (in-app inbox, Telegram), and (3) keep an auditable activity history per issue.

## Goals

1. **Live UI** — every mutation that affects a workspace is broadcast on the WS hub. The frontend rehydrates from REST after each connect, then drives off WS thereafter.
2. **Routed notifications** — an `inbox_item` is a per-recipient delivery with a severity (`action_required | attention | info`), an issue link, and read/archive state.
3. **Multi-channel** — inbox + Telegram today; channel selection is a per-user setting (plus per-workspace group chat).
4. **Per-issue activity log** merged with comments into a single timeline.
5. **Bulk inbox hygiene** — mark-all-read, archive-completed, archive-all-read.

## Non-Goals

- A general-purpose notification routing engine (Hookdeck / Knock / Courier-like).
- Email digests.
- Mobile push (no native apps).
- Slack integration (no handler yet; could be added behind the same notification listener pattern).

## Personas & user stories

- *As a user with the dashboard open*, I want to see comments appear without refreshing.
- *As a recipient of a task failure*, I want an inbox item that links straight to the failed task.
- *As a workspace admin*, I want the team Telegram group to receive a one-line notice when an `action_required` issue lands.
- *As a tech lead reviewing an issue history*, I want comments and status changes interleaved chronologically.

## Requirements

### WebSocket hub

| Req | Detail | Priority | Status |
|---|---|---|---|
| Single `/ws` endpoint | One connection per browser; rooms keyed by `workspace_id` | P0 | Shipped |
| Membership gate | Connect rejected if not a member of the requested workspace | P0 | Shipped |
| Hub broadcast | `Hub.Broadcast(workspaceID, event)` invoked from handlers and `TaskService` | P0 | Shipped |
| Auto-reconnect on client | `WSClient` reconnects with exponential backoff; resubscribes rooms | P0 | Shipped |
| Inbound message routing | Reserved but not used today (clients are read-only) | P1 | Planned |
| Cross-instance fanout | — | P2 | Future (Redis / NATS) |

### Event types (broadcast on `workspace_id` room)

| Event | Triggered by |
|---|---|
| `issue.created`, `issue.updated`, `issue.deleted` | Issue CRUD |
| `issue.batch_updated`, `issue.batch_deleted` | Bulk endpoints |
| `comment.created`, `comment.updated`, `comment.deleted` | Comment CRUD |
| `reaction.added`, `reaction.removed` | Reaction add / remove |
| `subscription.changed` | Subscribe / unsubscribe |
| `task.queued`, `task.dispatched`, `task.started`, `task.progress`, `task.message`, `task.completed`, `task.failed`, `task.cancelled` | Task lifecycle |
| `daemon.heartbeat`, `daemon.archived` | Daemon updates |
| `runtime.online`, `runtime.offline`, `runtime.usage` | Runtime updates |
| `inbox.created`, `inbox.read`, `inbox.archived` | Inbox events |
| `verification.phase_changed`, `criteria.updated` | Verification loop |

The frontend listens via `useWSEvent` and patches its zustand stores.

### Internal event bus

A separate in-process bus (`server/internal/events/`) lets server-side listeners react to mutations without coupling handlers to side effects.

| Listener | Purpose |
|---|---|
| Subscriber listener | Auto-subscribes creator/assignee/commenter to an issue |
| Inbox listener | Materializes `inbox_item` rows from events |
| Telegram listener | Posts to per-user channels and workspace group chats |
| Activity-log listener | Writes `activity_log` rows |
| Mention listener | Parses mentions and enqueues tasks (with chain accounting) |

### Inbox

| Req | Detail | Priority | Status |
|---|---|---|---|
| Per-recipient routing | `recipient_type ∈ {member, agent}` + `recipient_id` | P0 | Shipped |
| Item types | `issue_assigned`, `status_changed`, `new_comment`, `mention`, `task_failed`, `verification_blocked`, `criteria_ready`, etc. | P0 | Shipped |
| Severity | `action_required | attention | info` | P0 | Shipped |
| Title + body + issue link | Standard payload | P0 | Shipped |
| Read state | `read BOOLEAN` | P0 | Shipped |
| Archive state | `archived BOOLEAN` | P0 | Shipped |
| Details JSONB | Additional context per item type (since migration 019) | P0 | Shipped |
| Unread count | `GET /api/inbox/unread-count` | P0 | Shipped |
| Mark single read | `POST /api/inbox/{id}/read` | P0 | Shipped |
| Mark all read | `POST /api/inbox/mark-all-read` | P0 | Shipped |
| Archive single | `POST /api/inbox/{id}/archive` | P0 | Shipped |
| Archive all | `POST /api/inbox/archive-all` | P0 | Shipped |
| Archive all read | `POST /api/inbox/archive-all-read` | P0 | Shipped |
| Archive completed | `POST /api/inbox/archive-completed` — items whose linked issue is `done | cancelled` | P0 | Shipped |

### Activity log

| Req | Detail | Priority | Status |
|---|---|---|---|
| Per-issue, per-actor entries | `actor_type ∈ {member, agent, system}` | P0 | Shipped |
| Action types | `created, status_changed, assigned, unassigned, priority_changed, labels_changed, criteria_approved, verification_passed/failed, …` | P0 | Shipped |
| Details JSONB | Type-specific payload | P0 | Shipped |
| Timeline endpoint | `GET /api/issues/{id}/timeline` merges comments + activity log | P0 | Shipped |

### Telegram notifications

| Req | Detail | Priority | Status |
|---|---|---|---|
| Per-user channel | `user_notification_channel(channel_type='telegram', channel_id)` linked via bot DM (migration 053) | P0 | Shipped |
| Per-workspace group chat | `workspace.settings.telegram_group` with chat id and enabled flag | P0 | Shipped |
| Notification filters | Workspace setting picks which event types go to the group | P0 | Shipped |
| Server requires `TELEGRAM_BOT_TOKEN` env | — | P0 | Shipped |
| Slack / email channels | — | P2 | Future |

## Data model

```
inbox_item
├── id, workspace_id
├── recipient_type, recipient_id, actor_type, actor_id
├── type, severity, issue_id
├── title, body, details JSONB
├── read, archived, created_at

activity_log
├── id, workspace_id, issue_id
├── actor_type, actor_id
├── action, details JSONB
├── created_at

user_notification_channel
├── id, user_id, channel_type, channel_id
├── created_at
```

## Acceptance criteria

- Given an open dashboard, when another user updates an issue, then the change appears in the UI within 1s without manual refresh.
- Given a member assigned to an issue, when assignment occurs, then an `issue_assigned` inbox item is created with severity `attention`.
- Given a task failure, then a `task_failed` inbox item with severity `action_required` is created and routed to subscribers.
- Given a workspace with Telegram group enabled, when a configured event type fires, then a message is posted to the group with a link back to the issue.
- Given a user marking all inbox as read, when they reconnect on another device, then unread-count returns 0 and the UI reflects it.

## Open questions

- Should we **batch inbox items** for high-frequency events (e.g. 50 task messages in 30s)?
- Should the **activity log have a retention policy** (e.g. archive after N months)?
- Should **agent recipients** receive inbox items at all? Today the schema allows it; the rendering doesn't surface them.
- Should we add a **digest mode** — instead of one Telegram message per event, send a rollup every N minutes?
- Should the **timeline endpoint paginate** instead of returning all events for large issues?
