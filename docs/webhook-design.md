# Webhook System Design

## Overview

The webhook system allows Multica to receive events from external systems (alerting, monitoring, CI/CD, etc.) and take automated actions such as creating issues for AI agents to process.

The design separates **reception** (how we receive and parse external payloads) from **action** (what we do with the parsed event), making the system extensible for future use cases.

## Architecture

```
External System
       │
       ▼
POST /api/webhooks/{id}  (token-authenticated, no JWT)
       │
       ├─ 1. Authenticate via Bearer token
       ├─ 2. Look up webhook config (source_type, dedup_window, etc.)
       ├─ 3. Adapter.Parse(payload) → []Event  (normalize external format)
       ├─ 4. Per event:
       │     ├── Dedup check (webhook-level, configurable window)
       │     ├── Query enabled webhook_actions for this webhook
       │     └── Execute each action in order:
       │           ├── "create_issue" → create Issue + assign Agent + enqueue task
       │           └── (future: "close_issue", "comment", etc.)
       └─ 5. Return 202 Accepted with summary
```

## Data Model

### `webhook` table — Reception configuration

Defines an ingest endpoint. Each webhook has a unique URL and token.

| Column | Type | Description |
|---|---|---|
| `id` | UUID | Primary key, also part of the endpoint URL |
| `workspace_id` | UUID | Workspace scope |
| `name` | TEXT | Human-readable name (e.g. "Grafana Alerts") |
| `source_type` | TEXT | Adapter to use: `standard`, `oss-alert`, etc. |
| `token_hash` | TEXT | SHA-256 hash of the secret token |
| `token_prefix` | TEXT | First 12 chars of token for display |
| `status` | TEXT | `active` or `paused` |
| `dedup_window_seconds` | INT | Time window for deduplication (default 600) |
| `created_by` | UUID | User who created the webhook |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

### `webhook_action` table — Action configuration

Defines what to do when an event is received. Each webhook can have one or more actions (backend supports multiple; UI currently manages a single default action).

| Column | Type | Description |
|---|---|---|
| `id` | UUID | Primary key |
| `webhook_id` | UUID | FK to webhook |
| `action_type` | TEXT | `create_issue` (future: `close_issue`, `comment`) |
| `config` | JSONB | Action-specific parameters (see below) |
| `enabled` | BOOL | Whether this action is active |
| `position` | INT | Execution order (lower = first) |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

#### `config` schema for `action_type = "create_issue"`

```json
{
  "agent_id": "uuid",
  "title_template": "{{.title}}",
  "description_template": "{{.body}}",
  "labels": ["incident"],
  "dispatch_daemon_id": "uuid (optional)",
  "dispatch_daemon_label": "string (optional)"
}
```

Priority is not stored in the action config. It is resolved at event time: if the event payload contains a `priority` field, that value is used; otherwise defaults to `medium`.

### `webhook_event_log` table — Audit trail

Logs every received webhook invocation.

| Column | Type | Description |
|---|---|---|
| `id` | UUID | Primary key |
| `webhook_id` | UUID | FK to webhook |
| `dedup_key` | TEXT | Dedup identifier used |
| `payload` | JSONB | Raw incoming payload |
| `status` | TEXT | `processed`, `filtered`, `deduped`, `error` |
| `issue_id` | UUID | Created issue (if applicable) |
| `error_message` | TEXT | Error details (if applicable) |
| `created_at` | TIMESTAMPTZ | |

## Adapter System

### Normalized Event Format

Every adapter converts its source-specific payload into one or more `Event` structs:

```go
type Event struct {
    Type       string            // Event type: "alert.firing", "alert.resolved", "custom"
    DedupKey   string            // Deduplication identifier
    Data       map[string]string // All template variables (flat key-value)
    RawPayload json.RawMessage   // Original payload for audit
}
```

### Template Variable Resolution

Actions reference event data via `{{.key}}` placeholders. All variables come from `Event.Data` — a single flat namespace with no layered lookups.

**Naming convention to prevent conflicts:**

| Key type | Pattern | Example |
|---|---|---|
| Adapter-generated (deterministic) | Bare key | `title`, `body`, `alertname`, `app` |
| External user-defined data (from maps/labels) | Prefixed with source | `labels.xxx`, `annotations.xxx`, `fields.xxx` |

An adapter hardcodes a known set of bare keys and prefixes any dynamic keys from external data. This ensures external data can never shadow adapter-generated fields.

Example — oss-alert adapter output:

```go
Event{
    Type:     "alert.firing",
    DedupKey: "牵手后端服务panic:qianshou-marketing-rest",
    Data: map[string]string{
        // Adapter-generated (bare keys, deterministic)
        "title":         "牵手后端服务panic (qianshou-marketing-rest)",
        "body":          "**Started at:** 2026-04-13T07:39:46Z\n...",
        "alertname":     "牵手后端服务panic",
        "app":           "qianshou-marketing-rest",
        "generator_url": "https://prom-infra.p1.cn/...",

        // External user-defined data (prefixed, safe from conflicts)
        "labels.alertname":      "牵手后端服务panic",
        "labels.app":            "qianshou-marketing-rest",
        "annotations.value":     "1.2",
        "annotations.labels":    "map[app:...]",
    },
}
```

### Adapter Interface

```go
type Adapter interface {
    Parse(payload json.RawMessage, headers http.Header) ([]Event, error)
    Keys() []AdapterKey    // Declared keys for documentation and frontend hints
    Example() string       // Example JSON payload for frontend display
}

type AdapterKey struct {
    Key         string // e.g. "title", "alertname", "labels.*"
    Description string // Human-readable description
    Required    bool   // Whether the key is always present
}
```

The `GET /api/webhook-adapters` endpoint returns `AdapterInfo` for each registered adapter, including `source_type`, `description`, `keys`, and `example` payload. The frontend uses this to dynamically render schema documentation and template variable hints in the webhook creation/edit dialogs.

### Built-in Adapters

#### `standard` — Multica's own schema

For callers that can adapt to our format. The "no adapter" path.

**Input schema:**

```json
{
  "title": "Deployment failed",
  "body": "Deploy to production failed at step 3",
  "priority": "high",
  "dedup_key": "deploy-prod-v1.2.3",
  "fields": {
    "service": "api-gateway",
    "version": "v1.2.3"
  }
}
```

| Field | Required | Description |
|---|---|---|
| `title` | Yes | Event title |
| `body` | No | Detailed description (markdown) |
| `type` | No | Event type, defaults to `"custom"` |
| `dedup_key` | No | Deduplication identifier |
| `priority` | No | Priority hint (`urgent`, `high`, `medium`, `low`) |
| `fields` | No | Custom key-value pairs, available as `{{.fields.xxx}}` |

**Output Data keys:** `title`, `body`, `priority`, `fields.*`

#### `oss-alert` — Internal alert system (Prometheus AlertManager style)

For the internal alerting system that sends its own schema.

**Input schema:**

```json
{
  "labels": { "alertname": "ServicePanic", "app": "my-service" },
  "annotations": { "value": "1.2", "labels": "map[app:my-service]" },
  "startsAt": "2026-04-13T07:39:46.089Z",
  "endsAt": "2026-04-13T07:43:46.089Z",
  "generatorURL": "https://prometheus.example.com/graph?..."
}
```

**Output Data keys:** `title`, `body`, `alertname`, `app`, `generator_url`, `labels.*`, `annotations.*`

## API Endpoints

### Public (token-authenticated)

| Method | Path | Description |
|---|---|---|
| POST | `/api/webhooks/{id}` | Ingest webhook event |

### Protected (JWT + workspace membership)

| Method | Path | Description |
|---|---|---|
| GET | `/api/webhooks` | List webhooks in workspace |
| POST | `/api/webhooks` | Create webhook (+ default action) |
| GET | `/api/webhooks/{id}` | Get webhook details |
| PUT | `/api/webhooks/{id}` | Update webhook |
| DELETE | `/api/webhooks/{id}` | Delete webhook |
| POST | `/api/webhooks/{id}/regenerate-token` | Regenerate secret token |
| GET | `/api/webhooks/{id}/events` | List event log |
| GET | `/api/webhooks/{id}/actions` | List actions |
| POST | `/api/webhooks/{id}/actions` | Create action |
| PUT | `/api/webhooks/{id}/actions/{actionId}` | Update action |
| DELETE | `/api/webhooks/{id}/actions/{actionId}` | Delete action |
| GET | `/api/webhook-adapters` | List available adapters with keys and examples |

## Frontend UI

### Create Webhook Dialog

The dialog is divided into two visual sections:

1. **Endpoint** — Name, Source Type, and a collapsible schema info panel (adapter description, available template variables, example payload).
2. **Action** — Action Type selector (currently only `create_issue`), followed by action-specific fields rendered conditionally based on the selected type.

For `create_issue`, the action fields are: Agent, Environment, Issue Title Template, Issue Description Template.

### Edit Webhook Dialog

Same two-section layout as Create, pre-populated with existing values. Updates are sent as two API calls: `PUT /api/webhooks/{id}` for endpoint fields, `PUT /api/webhooks/{id}/actions/{actionId}` for action config. Token is not editable (use Regenerate Token separately).

### Webhook Card

Each webhook is shown as an expandable card with:
- Summary line: name, status badge, source type badge, agent name, event count, token prefix
- Action buttons: Edit, Pause/Activate, Regenerate Token, Delete
- Expanded details: URL, dedup window, and an "Action: Create Issue" sub-section showing agent, environment, and templates

### Current Simplifications

While the backend supports multiple actions per webhook, the UI currently manages only a single default action:

- Creating a webhook automatically creates one `create_issue` action.
- The edit dialog edits this single action inline.
- Action CRUD API exists for future extensibility when new action types are added.

## Token Authentication

Webhook tokens use the `whk_` prefix and are generated as `whk_` + 40 random hex characters. Tokens are hashed with SHA-256 before storage. The first 12 characters are stored as `token_prefix` for display purposes.

```
Authorization: Bearer whk_a1b2c3d4e5f6...
```

## Deduplication

Configured per webhook via `dedup_window_seconds` (default: 600 seconds / 10 minutes). When an event arrives with a `dedup_key` that matches a recently processed event within the window, it is logged as `deduped` and skipped.

## Issue Attribution

Issues created by webhooks use `creator_type = "webhook"` and `creator_id = webhook.id`. Webhook creators are not added as issue subscribers (only `member` and `agent` creator types are subscribed).

## Future Considerations

- **New action types:** `close_issue` (match and close existing issues), `comment` (add a comment to a matched issue). The frontend Action Type selector and conditional form rendering are already structured to support this.
- **Multi-action per webhook:** Backend supports it; UI will expose "Add Action" when there are multiple action types to choose from.
- **New adapters:** Add a Go file implementing the `Adapter` interface and register it in the adapter map. The frontend automatically picks it up via `GET /api/webhook-adapters`.
- **Event log viewer:** The `webhook_event_log` table and `GET /api/webhooks/{id}/events` API exist but are not yet surfaced in the UI.
