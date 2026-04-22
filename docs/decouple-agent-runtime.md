# Decouple Agent from Runtime

## Problem

Agent creation requires a `runtime_id`, permanently binding an agent to a specific runtime on a specific daemon. If that daemon goes offline, the agent is stuck. You can't load-balance across daemons or seamlessly switch providers.

## Design

### Agent gets `provider` instead of `runtime_id`

```
Before: Agent → runtime_id (UUID FK, NOT NULL) → one specific runtime on one machine
After:  Agent → provider (TEXT, e.g. "claude", "codex") → any matching online runtime
```

The agent no longer cares WHERE it runs, only WHAT provider it needs.

### Task dispatch: dynamic runtime selection

When a task is enqueued, the dispatcher picks a runtime:

```
1. Get agent.provider (e.g. "claude")
2. Find all online runtimes for this workspace + provider
3. Pick the one with the fewest running tasks (simple load balancing)
4. Set task.runtime_id = picked runtime
```

If no online runtime matches, the task stays queued (existing behavior for offline runtimes).

### `runtime_id` stays on tasks

`agent_task_queue.runtime_id` remains — it records which runtime actually executes the task. This is important for:
- Daemon polling (`ListPendingTasksByRuntime`)
- Usage tracking
- Offline task failure cleanup

### `runtime_mode` removed from Agent

Currently `agent.runtime_mode` is copied from the runtime at creation. With decoupling, this is unnecessary — all local daemons are "local", and if cloud runtimes are added later they'll be a different provider type.

## Migration

### DB Migration

```sql
-- Add provider column to agent (populated from runtime)
ALTER TABLE agent ADD COLUMN provider TEXT NOT NULL DEFAULT '';

-- Backfill from linked runtime
UPDATE agent a SET provider = rt.provider
FROM agent_runtime rt WHERE a.runtime_id = rt.id;

-- Make runtime_id nullable (no longer required)
ALTER TABLE agent ALTER COLUMN runtime_id DROP NOT NULL;

-- Drop the FK constraint (agent no longer requires a specific runtime)
ALTER TABLE agent DROP CONSTRAINT agent_runtime_id_fkey;
```

We keep `runtime_id` as a nullable column rather than dropping it — it can serve as an optional "preferred runtime" override. For now we ignore it in dispatch.

### New Query: dynamic runtime selection

```sql
-- name: FindAvailableRuntimeForProvider :one
SELECT ar.* FROM agent_runtime ar
WHERE ar.workspace_id = $1
  AND ar.provider = $2
  AND ar.status = 'online'
ORDER BY (
  SELECT COUNT(*) FROM agent_task_queue atq
  WHERE atq.runtime_id = ar.id AND atq.status IN ('queued', 'dispatched', 'running')
) ASC
LIMIT 1;
```

### Task Dispatch Change

In `server/internal/service/task.go` `enqueueTaskToAgent`:

```
Before:
  task.runtime_id = agent.RuntimeID  (hard-coded from agent)

After:
  runtime = findAvailableRuntime(workspace_id, agent.Provider)
  task.runtime_id = runtime.ID
```

### Handler Changes

- `CreateAgent`: accept `provider` (required) instead of `runtime_id`
- `UpdateAgent`: accept optional `provider`, remove runtime_id update
- `AgentResponse`: replace `runtime_id` with `provider`
- issue.go/comment.go: change `agent.RuntimeID.Valid` guard to `agent.Provider != ""`

### Frontend Changes

- `Agent` type: `provider: string` instead of `runtime_id: string`
- Agent creation: simple provider dropdown instead of daemon/runtime two-step picker
- Agent detail header: show provider directly, no runtime lookup
- Agent list: provider shown directly on each item

## What stays the same

- `agent_task_queue.runtime_id` — tasks still track which runtime executes them
- Daemon heartbeat/polling — daemons still claim tasks by runtime_id
- Runtime usage tracking — per-runtime
- Task lifecycle (start/complete/fail) — unchanged
