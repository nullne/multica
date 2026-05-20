# Tasks & Verification Loop

## Problem

When an agent is assigned an issue, the system has to (1) reliably dispatch the work to a daemon, (2) stream the agent's output back so humans can watch progress, (3) detect failure and surface it, (4) optionally enforce a verification gate before the issue is marked done. All of this must be observable per task, restartable, and cancellable.

## Goals

1. **Pull-based, NAT-friendly dispatch** — daemons poll, server never has to reach into a user's network.
2. **Streaming output** — every line of agent text, tool use, and tool result is persisted and broadcast.
3. **Lifecycle observability** — `queued → dispatched → running → completed | failed | cancelled` transitions are recorded with timestamps, errors, and a result blob.
4. **Verification loop (optional)** — when a verifier agent is configured, the system orchestrates `criteria → executor → validator → rework` until pass / blocked.
5. **Per-issue activity history** — every run for an issue is listed; messages can be incrementally fetched.

## Non-Goals

- Distributed task queues (Celery / RabbitMQ). The DB-backed queue is sufficient at current scale.
- A generic workflow engine. The verification loop is hand-coded for this specific flow.
- Live editing of in-progress runs.

## Personas & user stories

- *As an agent operator*, I want a task to be dispatched within a few seconds of assignment.
- *As a reviewer*, I want to watch the agent's output stream as it runs.
- *As a tech lead*, I want to cancel a runaway task with one click.
- *As an admin*, I want failed tasks to show up in the inbox with the error message.
- *As a tech lead*, I want acceptance criteria to be checked by a different agent before an issue is closed.

## Requirements

### Lifecycle

| State | Set by | Meaning |
|---|---|---|
| `queued` | Enqueue (assignment, mention, schedule) | Awaiting a runtime to claim it. |
| `dispatched` | Daemon `ClaimTaskByRuntime` | A runtime has the task but the agent CLI hasn't reported in yet. |
| `running` | Daemon `POST /tasks/{id}/start` | Agent CLI is executing. `started_at` is recorded. |
| `completed` | Daemon `POST /tasks/{id}/complete` | Success; `result JSONB` is persisted. |
| `failed` | Daemon `POST /tasks/{id}/fail` or server timeout | `error TEXT` is persisted. |
| `cancelled` | User `POST /issues/{id}/tasks/{taskId}/cancel` | User intent; daemon is told on next poll. |

### Dispatch

| Req | Detail | Priority | Status |
|---|---|---|---|
| Enqueue on assign | `assignee_type=agent` triggers `enqueueTaskToAgent` | P0 | Shipped |
| Enqueue on mention | Comment with `@AgentName` enqueues a task with `trigger_comment_id` | P0 | Shipped |
| Enqueue on schedule | Recurring template creates the issue + enqueues a task | P0 | Shipped |
| Dynamic runtime selection | Pick least-loaded online runtime matching `agent.providers` | P0 | Shipped |
| Honor max concurrency | `agent.max_concurrent_tasks` is enforced at dispatch | P0 | Shipped |
| Lifecycle guards | DB triggers prevent illegal state transitions (migration 022) | P0 | Shipped |
| Per-task `context` JSONB | Carries `flow`, `role`, `round`, `executor_agent_id`, `verifier_agent_id`, `source_task_id` | P0 | Shipped |
| Task session ID | `session_id` (migration 020) — opaque identifier for the agent's session | P0 | Shipped |
| Trigger comment | `trigger_comment_id` (migration 028) — set for mention/comment-triggered tasks | P0 | Shipped |
| Cancel | `POST /api/issues/{id}/tasks/{taskId}/cancel` | P0 | Shipped |

### Streaming messages

| Req | Detail | Priority | Status |
|---|---|---|---|
| Append message | `POST /api/daemon/tasks/{id}/messages` — body is a list of typed messages | P0 | Shipped |
| Message types | `text | tool_use | tool_result | error | thinking` (extensible) | P0 | Shipped |
| Sequence number | Every message gets a monotonic `seq` for incremental fetch | P0 | Shipped |
| List messages | `GET /api/daemon/tasks/{id}/messages?since=N` (also exposed to clients) | P0 | Shipped |
| WS broadcast | Every appended message is broadcast on the workspace room | P0 | Shipped |
| Progress report | `POST /api/daemon/tasks/{id}/progress` for percentage / step hints | P0 | Shipped |

### Verification loop

When `issue.verifier_agent_id` is set and the issue is assigned to an executor agent, the system follows the loop in [`docs/issue-verification-loop-design.md`](../../issue-verification-loop-design.md).

| State | Meaning |
|---|---|
| `none` | Verification disabled (default) — flow is the same as today. |
| `criteria_pending` | Verifier was asked to generate criteria; awaiting completion. |
| `execution_pending` | Criteria are approved; executor task is queued. |
| `validating` | Executor finished; validator is checking. |
| `rework_pending` | Validator said `fail`; rework task is queued to the executor. |
| `passed` | Validator said `pass`; issue may auto-move to `done` if currently `in_review`. |
| `blocked` | Too many failed rounds (default 5) or an unrecoverable error. |

| Req | Detail | Priority | Status |
|---|---|---|---|
| `verifier_agent_id` on issue | Distinct from `assignee_id`; verifier ≠ assignee | P0 | Shipped |
| `verification_phase` state machine | Enum above, persisted | P0 | Shipped |
| `verification_round` counter | Increments on each fail | P0 | Shipped |
| `last_verification_result JSONB` | `{ decision, summary, failed_checks[] }` | P0 | Shipped |
| `max_verification_rounds` per issue | Default 5 | P0 | Shipped |
| Criteria approve / reject | `POST /api/issues/{id}/criteria/approve` and `/reject` (human gate) | P0 | Shipped |
| Update criteria | `PUT /api/issues/{id}/criteria` | P0 | Shipped |
| Structured prompt protocol | Agents emit JSON inside `<!--multica:criteria … -->` / `<!--multica:verification … -->` comment blocks; server parses | P0 | Shipped |
| Per-workspace max-rounds override | — | P1 | Gap (today fixed/per-issue) |
| Verifier identity for system comments | Verifier agent is the comment author when reporting pass/fail | P0 | Shipped |

### Listing & filtering

| Req | Endpoint | Priority | Status |
|---|---|---|---|
| Active tasks per workspace | `GET /api/tasks/active` | P0 | Shipped |
| Active task per issue | `GET /api/issues/{id}/active-task` | P0 | Shipped |
| Run history per issue | `GET /api/issues/{id}/task-runs` | P0 | Shipped |
| Tasks per agent | `GET /api/agents/{id}/tasks` | P0 | Shipped |

## Data model

```
agent_task_queue
├── id, agent_id, issue_id, runtime_id
├── status, priority
├── trigger_comment_id (nullable)
├── context JSONB ({flow, role, round, executor_agent_id, verifier_agent_id, source_task_id})
├── session_id
├── dispatched_at, started_at, completed_at
├── result JSONB, error TEXT
├── created_at

task_message
├── id, task_id, seq
├── type (text | tool_use | tool_result | error | thinking)
├── content JSONB, created_at

runtime_usage
├── runtime_id, date, model, input_tokens, output_tokens, cache_*  (see runtimes-and-daemons)
```

## API surface

### Server → user

```
GET    /api/tasks/active
GET    /api/issues/{id}/active-task
GET    /api/issues/{id}/task-runs
GET    /api/agents/{id}/tasks
POST   /api/issues/{id}/tasks/{taskId}/cancel
PUT    /api/issues/{id}/criteria
POST   /api/issues/{id}/criteria/approve
POST   /api/issues/{id}/criteria/reject
```

### Daemon → server (`/api/daemon/*`)

```
POST   /api/daemon/runtimes/{runtimeId}/tasks/claim
GET    /api/daemon/runtimes/{runtimeId}/tasks/pending
GET    /api/daemon/tasks/{taskId}/status
POST   /api/daemon/tasks/{taskId}/start
POST   /api/daemon/tasks/{taskId}/progress
POST   /api/daemon/tasks/{taskId}/complete
POST   /api/daemon/tasks/{taskId}/fail
POST   /api/daemon/tasks/{taskId}/messages
GET    /api/daemon/tasks/{taskId}/messages
```

## Acceptance criteria

- Given a freshly enqueued task, when a daemon polls and claims it, the row moves to `dispatched` and `dispatched_at` is set.
- Given a task that has been `running` for longer than `MULTICA_AGENT_TIMEOUT` (default 2h), the system marks it `failed` with a timeout error.
- Given a cancelled task, the next time the daemon polls, the server returns the task as cancelled and the daemon stops the agent.
- Given an issue with verifier configured and empty criteria, when assigned, the criteria task is enqueued (not the executor).
- Given a validator task whose output contains `<!--multica:verification {"decision":"fail",…} -->`, when completed, the issue moves to `rework_pending` and a rework task is enqueued — unless `verification_round + 1 > max_verification_rounds`, in which case the issue moves to `blocked`.

## Open questions

- Should we **persist task `cancelled_by`** (user id) for audit?
- Should validator output parsing tolerate **YAML or JSON inside fenced code blocks** in addition to comment markers?
- Should `verification_phase = blocked` trigger an `action_required` inbox item to the workspace admins?
- Should the executor receive the **failed_checks list** as structured context, not just text in the next prompt?
- Should `max_verification_rounds` be configurable at the workspace level, not just per issue?
