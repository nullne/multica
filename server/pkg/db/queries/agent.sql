-- name: ListAgents :many
SELECT * FROM agent
WHERE workspace_id = $1 AND archived_at IS NULL
ORDER BY created_at ASC;

-- name: ListAllAgents :many
SELECT * FROM agent
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: GetAgent :one
SELECT * FROM agent
WHERE id = $1;

-- name: GetAgentInWorkspace :one
SELECT * FROM agent
WHERE id = $1 AND workspace_id = $2;

-- name: CreateAgent :one
INSERT INTO agent (
    workspace_id, name, description, avatar_url, providers, visibility, owner_id,
    tools, triggers, instructions, github_code_access, default_provider, default_daemon_id, max_concurrent_tasks, model_config
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, sqlc.narg(default_provider), sqlc.narg(default_daemon_id), @max_concurrent_tasks, @model_config)
RETURNING *;

-- name: UpdateAgent :one
UPDATE agent SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    providers = COALESCE(sqlc.narg('providers'), providers),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    status = COALESCE(sqlc.narg('status'), status),
    tools = COALESCE(sqlc.narg('tools'), tools),
    triggers = COALESCE(sqlc.narg('triggers'), triggers),
    instructions = COALESCE(sqlc.narg('instructions'), instructions),
    github_code_access = COALESCE(sqlc.narg('github_code_access'), github_code_access),
    default_provider = sqlc.narg('default_provider'),
    default_daemon_id = sqlc.narg('default_daemon_id'),
    max_concurrent_tasks = COALESCE(sqlc.narg('max_concurrent_tasks'), max_concurrent_tasks),
    model_config = COALESCE(sqlc.narg('model_config'), model_config),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveAgent :one
UPDATE agent SET archived_at = now(), archived_by = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RestoreAgent :one
UPDATE agent SET archived_at = NULL, archived_by = NULL, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListAgentTasks :many
SELECT * FROM agent_task_queue
WHERE agent_id = $1
ORDER BY created_at DESC;

-- name: CreateAgentTask :one
INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, trigger_comment_id, context, provider)
VALUES ($1, $2, $3, 'queued', $4, sqlc.narg(trigger_comment_id), sqlc.narg(context), sqlc.narg(provider))
RETURNING *;

-- name: CancelAgentTasksByIssue :exec
UPDATE agent_task_queue
SET status = 'cancelled'
WHERE issue_id = $1 AND status IN ('queued', 'dispatched', 'running');

-- name: CancelAgentTasksByAgent :exec
UPDATE agent_task_queue
SET status = 'cancelled'
WHERE agent_id = $1 AND status IN ('queued', 'dispatched', 'running');

-- name: GetAgentTask :one
SELECT * FROM agent_task_queue
WHERE id = $1;

-- name: ClaimAgentTask :one
-- Claims the next queued task for an agent, enforcing per-issue serialization and
-- the per-agent concurrency cap: a task is claimable only when no other task for
-- the same issue is already dispatched or running, and the agent's total active
-- task count is below its max_concurrent_tasks limit.
UPDATE agent_task_queue
SET status = 'dispatched', dispatched_at = now()
WHERE id = (
    SELECT atq.id FROM agent_task_queue atq
    WHERE atq.agent_id = $1 AND atq.status = 'queued'
      AND NOT EXISTS (
          SELECT 1 FROM agent_task_queue active
          WHERE active.issue_id = atq.issue_id
            AND active.status IN ('dispatched', 'running')
      )
      AND (
          SELECT COUNT(*) FROM agent_task_queue running
          WHERE running.agent_id = $1
            AND running.status IN ('dispatched', 'running')
      ) < (SELECT max_concurrent_tasks FROM agent WHERE id = $1)
    ORDER BY atq.priority DESC, atq.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: StartAgentTask :one
UPDATE agent_task_queue
SET status = 'running', started_at = now()
WHERE id = $1 AND status = 'dispatched'
RETURNING *;

-- name: CompleteAgentTask :one
UPDATE agent_task_queue
SET status = 'completed',
    completed_at = now(),
    result = $2,
    session_id = $3,
    work_dir = $4,
    model = COALESCE(sqlc.narg(model), model),
    thinking_level = COALESCE(sqlc.narg(thinking_level), thinking_level)
WHERE id = $1 AND status = 'running'
RETURNING *;

-- name: SetAgentTaskExecutionMetadata :one
UPDATE agent_task_queue
SET provider = COALESCE(sqlc.narg(provider), provider),
    model = COALESCE(sqlc.narg(model), model),
    thinking_level = COALESCE(sqlc.narg(thinking_level), thinking_level)
WHERE id = $1 AND status IN ('queued', 'dispatched', 'running')
RETURNING *;

-- name: SetAgentTaskResultComment :one
-- Links a task to the comment it produced as its final reply. Only sets if
-- not already linked, so callers can safely call this even if a comment was
-- created in a transient path that already wrote the link.
UPDATE agent_task_queue
SET result_comment_id = $2
WHERE id = $1 AND result_comment_id IS NULL
RETURNING *;

-- name: GetLastTaskSession :one
-- Returns the session_id and work_dir from the most recent completed task
-- for a given (agent_id, issue_id) pair, used for session resumption.
SELECT session_id, work_dir FROM agent_task_queue
WHERE agent_id = $1 AND issue_id = $2 AND status = 'completed' AND session_id IS NOT NULL
ORDER BY completed_at DESC
LIMIT 1;

-- name: FailAgentTask :one
UPDATE agent_task_queue
SET status = 'failed', completed_at = now(), error = $2
WHERE id = $1 AND status IN ('dispatched', 'running')
RETURNING *;

-- name: FailStaleTasks :many
-- Fails tasks stuck in dispatched/running beyond the given thresholds.
-- Handles cases where the daemon is alive but the task is orphaned
-- (e.g. agent process hung, daemon failed to report completion).
UPDATE agent_task_queue
SET status = 'failed', completed_at = now(), error = 'task timed out'
WHERE (status = 'dispatched' AND dispatched_at < now() - make_interval(secs => @dispatch_timeout_secs::double precision))
   OR (status = 'running' AND started_at < now() - make_interval(secs => @running_timeout_secs::double precision))
RETURNING id, agent_id, issue_id;

-- name: CancelAgentTask :one
UPDATE agent_task_queue
SET status = 'cancelled', completed_at = now()
WHERE id = $1 AND status IN ('queued', 'dispatched', 'running')
RETURNING *;

-- name: CountRunningTasks :one
SELECT count(*) FROM agent_task_queue
WHERE agent_id = $1 AND status IN ('dispatched', 'running');

-- name: HasActiveTaskForIssue :one
-- Returns true if there is any queued, dispatched, or running task for the issue.
SELECT count(*) > 0 AS has_active FROM agent_task_queue
WHERE issue_id = $1 AND status IN ('queued', 'dispatched', 'running');

-- name: HasPendingTaskForIssue :one
-- Returns true if there is a queued or dispatched (but not yet running) task for the issue.
-- Used by the coalescing queue: allow enqueue when a task is running (so
-- the agent picks up new comments on the next cycle) but skip if a pending
-- task already exists (natural dedup).
SELECT count(*) > 0 AS has_pending FROM agent_task_queue
WHERE issue_id = $1 AND status IN ('queued', 'dispatched');

-- name: HasPendingTaskForIssueAndAgent :one
-- Returns true if a specific agent already has a queued or dispatched task
-- for the given issue. Used by @mention trigger dedup.
SELECT count(*) > 0 AS has_pending FROM agent_task_queue
WHERE issue_id = $1 AND agent_id = $2 AND status IN ('queued', 'dispatched');

-- name: ListPendingTasksByRuntime :many
SELECT * FROM agent_task_queue
WHERE runtime_id = $1 AND status IN ('queued', 'dispatched')
ORDER BY priority DESC, created_at ASC;

-- name: ListActiveTasksByIssue :many
SELECT * FROM agent_task_queue
WHERE issue_id = $1 AND status IN ('dispatched', 'running')
ORDER BY created_at DESC;

-- name: ListActiveTasksByWorkspace :many
SELECT atq.* FROM agent_task_queue atq
JOIN issue i ON atq.issue_id = i.id
WHERE i.workspace_id = $1 AND atq.status IN ('queued', 'dispatched', 'running')
ORDER BY atq.created_at DESC;

-- name: ListTasksByIssue :many
SELECT * FROM agent_task_queue
WHERE issue_id = $1
ORDER BY created_at DESC;

-- name: UpdateAgentStatus :one
UPDATE agent SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;
