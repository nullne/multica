# Agents

## Problem

The core differentiator of Multica is the **agent as teammate** model. Agents must look and behave enough like a member to be assignable from the board, but they also need fields a human doesn't have — instructions, providers, triggers, skills, concurrency limits, archival state.

## Goals

1. **Symmetric with members** on the board, in comments, in reactions, in the inbox.
2. **Provider-agnostic** — an agent declares "I run on `claude_code` or `codex`", not "I run on daemon X." Dispatch finds an available runtime at task time (see [decouple-agent-runtime.md](../../decouple-agent-runtime.md)).
3. **Triggers** are first-class — assignment, comment, mention each map to a configurable trigger.
4. **Loop-safe** — mention-chain accounting prevents two agents from ping-ponging forever.
5. **Reversible lifecycle** — agents can be archived and restored without losing history.

## Non-Goals

- A marketplace of pre-built agents.
- Agent-to-agent direct messaging outside an issue thread.
- Per-agent rate limits beyond `max_concurrent_tasks`.
- A visual workflow editor for triggers.

## Personas & user stories

- *As a tech lead*, I want to create an agent named "Lambda" that uses Claude Code and has read access to our GitHub.
- *As an admin*, I want to attach the "deploy" skill to Lambda so she knows our release runbook.
- *As an engineer*, I want to assign issues to Lambda and have her pick them up on whichever of my daemons is online.
- *As a reviewer*, I want to `@Lambda` in a comment thread and have her respond by working in that thread.
- *As an owner*, I want to archive an agent without breaking its old issues.

## Requirements

### Agent entity

| Req | Detail | Priority | Status |
|---|---|---|---|
| Workspace-scoped | `workspace_id` FK | P0 | Shipped |
| Name, description, avatar | Standard profile fields | P0 | Shipped |
| Providers (multi) | `providers TEXT[]` (e.g. `["claude_code", "codex"]`) | P0 | Shipped |
| Default provider | `default_provider` (nullable) — pinned per agent | P0 | Shipped |
| Default daemon | `default_daemon_id` (nullable) — optional preferred runtime owner | P0 | Shipped |
| Instructions | `instructions TEXT` — system prompt prepended to every run (migration 021) | P0 | Shipped |
| Tools | `tools JSONB` — per-agent allow/deny list for CLI tool use | P0 | Shipped |
| Triggers | `triggers JSONB` — `on_assign`, `on_comment`, `on_mention` configuration | P0 | Shipped |
| Concurrency | `max_concurrent_tasks INT` (default 6 since migration 023/052) | P0 | Shipped |
| Visibility | `workspace | private` (default `private` since migration 030) | P0 | Shipped |
| Status | `idle | working | blocked | error | offline` | P0 | Shipped |
| GitHub code access | `github_code_access ∈ {read, write, admin}` (migration 032) | P0 | Shipped |
| Owner | `owner_id` FK to user | P0 | Shipped |
| Archive / restore | `archived_at`, `archived_by`; `POST .../archive` and `.../restore` (migration 031) | P0 | Shipped |
| Bot users | Separate `kind='bot'` users (not agents) for system-authored comments — see workspaces appendix | P0 | Shipped |

### Triggers

| Trigger | What fires | Where to configure | Notes |
|---|---|---|---|
| `on_assign` | Agent receives an issue assignment | `triggers.on_assign` | Dispatches an executor task immediately (or a criteria task if a verifier is configured and criteria are empty). |
| `on_comment` | New comment on an issue the agent is subscribed to | `triggers.on_comment` | Used for agents that should react to thread activity even without a direct mention. |
| `on_mention` | Agent is `@mentioned` in a comment | `triggers.on_mention` | Bounded by the mention-chain cap on the issue. |

> Scheduled triggers are not part of agent configuration — use **recurring templates** (see [recurring-templates.md](recurring-templates.md)) which assign an agent on creation.

### CRUD & lifecycle

| Req | Endpoint | Priority | Status |
|---|---|---|---|
| List agents | `GET /api/agents` | P0 | Shipped |
| Create agent | `POST /api/agents` (admin/owner) | P0 | Shipped |
| Get agent | `GET /api/agents/{id}` | P0 | Shipped |
| Update agent | `PUT /api/agents/{id}` | P0 | Shipped |
| Archive | `POST /api/agents/{id}/archive` | P0 | Shipped |
| Restore | `POST /api/agents/{id}/restore` | P0 | Shipped |
| List tasks | `GET /api/agents/{id}/tasks` | P0 | Shipped |
| Get skills | `GET /api/agents/{id}/skills` | P0 | Shipped |
| Set skills | `PUT /api/agents/{id}/skills` | P0 | Shipped |

### Dispatch rules

1. On task enqueue, the dispatcher reads `agent.providers` (or `default_provider`).
2. It finds online runtimes in the same workspace whose `provider` is in that list.
3. It picks the runtime with the **fewest active tasks** (queued + dispatched + running) — simple least-loaded.
4. If `agent.default_daemon_id` is set, prefer runtimes on that daemon when otherwise equal.
5. If no runtime is online, the task stays `queued` until one comes online.

### Visibility

- `private`: only the agent's `owner_id` can assign / `@mention` it. Default for newly created agents.
- `workspace`: any member can assign / `@mention` it.

## Data model

```
agent
├── id, workspace_id, name, description, avatar_url
├── providers TEXT[], default_provider, default_daemon_id
├── instructions TEXT, tools JSONB, triggers JSONB
├── visibility, status, max_concurrent_tasks
├── github_code_access
├── owner_id, archived_at, archived_by
├── created_at, updated_at

agent_skill (M:N → skill)
agent_task_queue (1:N → task)
```

## Acceptance criteria

- Given an agent with `providers=["claude_code"]`, when a task is enqueued and the only online runtime is `codex`, then the task stays `queued`.
- Given an agent with two online runtimes both for `claude_code`, the dispatcher picks the runtime with fewer active tasks.
- Given an agent with `visibility=private`, when a non-owner tries to assign an issue to it, then the request is rejected.
- Given an archived agent, when a user tries to assign it, then the assignment is rejected and the agent does not appear in the assignee picker.
- Given an agent at `max_concurrent_tasks`, when a new task is enqueued, then the task remains `queued` until a slot frees.

## Open questions

- Should `default_daemon_id` become a hard pin (always run here, fail if offline) in addition to today's soft preference?
- Should agents support per-trigger `enabled` flags via `triggers` JSONB explicitly (today the trigger is "configured = enabled")?
- Should the `github_code_access` field be enforced server-side (today it is informational; the agent CLI honors it via its own auth)?
- Should `private` visibility be the default forever, or should workspaces opt into "agents are public by default" via settings?
