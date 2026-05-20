# Recurring Issue Templates

## Problem

Some work is recurring on a schedule (daily standup recap, weekly health-check, end-of-sprint cleanup) and should land on the agent queue without a human kicking it off. The product needs a primitive that creates a new issue on a cron schedule, with the same assignee, labels, and dispatch hints each time.

## Goals

1. **Cron-driven issue creation** with timezone support — recurring work fires at the user's local "9am every weekday."
2. **Same template, many issues** — every fire creates a fresh issue (not a re-open of the same one) so each occurrence has its own thread.
3. **Optional bounding** — `max_runs` lets a template stop after N occurrences without manual cleanup.
4. **Per-template subscribers** — known interested members are auto-subscribed to every issue created from the template.
5. **Reliable single-fire** — a tick that occurs while a server is multi-replica must not produce duplicates.

## Non-Goals

- Generic workflow scheduling beyond "create one issue."
- Multiple actions per fire (each template creates one issue today).
- Conditional triggers ("only fire if X is true").
- Editing already-created issues from a template change.

## Personas & user stories

- *As a tech lead*, I want a "weekly metrics review" issue created every Monday at 9am, assigned to our analytics agent.
- *As an admin*, I want to pause a recurring template without losing its config.
- *As an admin*, I want a template that runs four times then stops (e.g. a campaign).
- *As a stakeholder*, I want to be auto-subscribed to every issue from a template I care about.

## Requirements

### Template entity

| Field | Type | Notes |
|---|---|---|
| `id`, `workspace_id` | UUID | |
| `name` | TEXT | Human label, not the issue title |
| `cron` | TEXT | 5-field cron expression |
| `timezone` | TEXT | Default `UTC`; e.g. `America/New_York` |
| `title`, `description`, `priority` | — | Used to create each issue |
| `assignee_type`, `assignee_id` | polymorphic | Optional (member or agent) |
| `due_date_offset_hours` | INT | Relative due date; nullable |
| `dispatch_provider`, `dispatch_daemon_id`, `dispatch_daemon_label` | — | Optional dispatch hints (mirrors webhook action config) |
| `max_runs` | INT NULL | If set, stop creating after this many fires (migration 056) |
| `runs_count` | INT | Monotonic counter |
| `status` | `active | paused` | |
| `created_by`, `created_at`, `updated_at` | | |

### Subscriber list

`recurring_template_subscriber(template_id, user_id)` — when an issue is created, each listed user is auto-added to the new issue's subscriber list, so their inbox sees it (migration 059).

### Scheduling

| Req | Detail | Priority | Status |
|---|---|---|---|
| In-process scheduler | `cmd/server/recurring_scheduler.go` ticks every 60s | P0 | Shipped |
| Optimistic claim | `ClaimRecurringTemplate` row update guards against duplicate fires across replicas | P0 | Shipped |
| Skip if paused | `status != 'active'` | P0 | Shipped |
| Skip if `max_runs` reached | `runs_count >= max_runs` (when set) | P0 | Shipped |
| Honor timezone | Cron evaluated in the template's TZ | P0 | Shipped |
| Audit per fire | `runs_count` increments; created issue id is correlated via creator metadata | P0 | Shipped |

### CRUD endpoints

```
GET    /api/recurring-templates
POST   /api/recurring-templates                  (admin)
GET    /api/recurring-templates/{id}
PATCH  /api/recurring-templates/{id}             (admin)
DELETE /api/recurring-templates/{id}             (admin)
```

## Data model

```
recurring_issue_template
├── id, workspace_id, name
├── cron, timezone
├── title, description, priority
├── assignee_type, assignee_id
├── due_date_offset_hours
├── dispatch_provider, dispatch_daemon_id, dispatch_daemon_label
├── status, max_runs, runs_count
├── created_by, created_at, updated_at

recurring_template_subscriber
├── template_id, user_id
```

## Acceptance criteria

- Given an `active` template with cron `0 9 * * MON-FRI` and tz `America/New_York`, when the clock reaches 9:00am NYC time on a weekday, then exactly one new issue is created — even if two server instances tick concurrently.
- Given a template with `max_runs=4` and `runs_count=4`, when the next tick fires, then no issue is created and the template is implicitly retired (no row deletion).
- Given a template with two subscribers, when an issue is created, then both users appear in the new issue's `issue_subscriber` list.
- Given a template with `assignee_type='agent'` and `assignee_id=X`, when the issue is created, then a task is enqueued to agent X (same path as a manual assignment).
- Given a paused template, when the cron time matches, then no issue is created.

## Open questions

- Should a template support **multiple cron schedules** (e.g. daily and on the last day of the month) without splitting into two templates?
- Should we offer a **"create if no open issue exists"** mode so a forgotten recurring issue doesn't pile up?
- Should `runs_count` be **reset by re-activating** a paused template, or carry over?
- Should the scheduler emit a **`recurring.fired` event** for observability (Telegram digest of "today's recurring issues")?
- Should a template support **draft mode** — create the issue but leave it `backlog` and unassigned, requiring human review before agent execution?
