-- name: ListIssues :many
SELECT * FROM issue
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority'))
  AND (sqlc.narg('assignee_id')::uuid IS NULL OR assignee_id = sqlc.narg('assignee_id'))
ORDER BY position ASC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListRecentIssues :many
-- Returns the most recently updated issues in a workspace, optionally limited
-- to ones the supplied user is involved in (creator or assignee). Used to
-- populate the home sidebar Recents list — backed by updated_at so the list
-- reflects actual recency without client-side reordering.
SELECT * FROM issue
WHERE workspace_id = $1
  AND (
    sqlc.narg('user_id')::uuid IS NULL
    OR creator_id = sqlc.narg('user_id')
    OR assignee_id = sqlc.narg('user_id')
  )
ORDER BY updated_at DESC
LIMIT $2;

-- name: GetIssue :one
SELECT * FROM issue
WHERE id = $1;

-- name: GetIssueInWorkspace :one
SELECT * FROM issue
WHERE id = $1 AND workspace_id = $2;

-- name: CreateIssue :one
INSERT INTO issue (
    workspace_id, title, description, status, priority,
    assignee_type, assignee_id, creator_type, creator_id,
    verifier_agent_id, parent_issue_id, position, due_date, number,
    max_verification_rounds,
    dispatch_provider, dispatch_daemon_id, dispatch_daemon_label,
    github_auto_fix_enabled, dispatch_after
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
    $16, $17, $18, $19, $20
) RETURNING *;

-- name: GetIssueByNumber :one
SELECT * FROM issue
WHERE workspace_id = $1 AND number = $2;

-- name: UpdateIssue :one
UPDATE issue SET
    title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    status = COALESCE(sqlc.narg('status'), status),
    priority = COALESCE(sqlc.narg('priority'), priority),
    assignee_type = sqlc.narg('assignee_type'),
    assignee_id = sqlc.narg('assignee_id'),
    verifier_agent_id = sqlc.narg('verifier_agent_id'),
    parent_issue_id = sqlc.narg('parent_issue_id'),
    position = COALESCE(sqlc.narg('position'), position),
    due_date = sqlc.narg('due_date'),
    max_verification_rounds = sqlc.narg('max_verification_rounds'),
    dispatch_provider = sqlc.narg('dispatch_provider'),
    dispatch_daemon_id = sqlc.narg('dispatch_daemon_id'),
    dispatch_daemon_label = sqlc.narg('dispatch_daemon_label'),
    github_auto_fix_enabled = COALESCE(sqlc.narg('github_auto_fix_enabled'), github_auto_fix_enabled),
    -- Reset the one-shot fired marker whenever dispatch_after changes, so a
    -- rescheduled time becomes eligible for dispatch again.
    dispatch_after_fired_at = CASE
        WHEN sqlc.narg('dispatch_after')::timestamptz IS DISTINCT FROM dispatch_after THEN NULL
        ELSE dispatch_after_fired_at
    END,
    dispatch_after = sqlc.narg('dispatch_after'),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListIssuesByParent :many
SELECT * FROM issue
WHERE parent_issue_id = $1
ORDER BY position ASC, created_at DESC;

-- name: UpdateIssueAcceptanceCriteria :one
UPDATE issue SET
    acceptance_criteria = $2,
    criteria_status = sqlc.narg('criteria_status'),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateIssueCriteriaStatus :one
UPDATE issue SET
    criteria_status = sqlc.narg('criteria_status'),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateIssueStatus :one
UPDATE issue SET
    status = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteIssue :exec
DELETE FROM issue WHERE id = $1;

-- name: TryReserveIssueAgentMentionChain :one
-- Atomically reserves one slot in the agent-to-agent mention chain for this
-- issue. Returns the new count and the current generation when the reservation
-- succeeds. Returns no rows (pgx.ErrNoRows) when the chain has already reached
-- the supplied limit, so concurrent callers cannot exceed the cap.
UPDATE issue SET
    agent_mention_chain_count = agent_mention_chain_count + 1
WHERE id = $1 AND agent_mention_chain_count < sqlc.arg(max_count)::int
RETURNING agent_mention_chain_count, agent_mention_chain_generation;

-- name: ReleaseIssueAgentMentionChainSlot :execrows
-- Releases a previously-reserved chain slot. The generation match ensures we
-- only roll back when the chain still belongs to the same epoch as the
-- reservation — if a human reset has bumped the generation in between, this
-- becomes a no-op so the post-reset chain is not corrupted.
UPDATE issue SET
    agent_mention_chain_count = agent_mention_chain_count - 1
WHERE id = $1
  AND agent_mention_chain_generation = sqlc.arg(generation)::bigint
  AND agent_mention_chain_count > 0;

-- name: ListDueDispatchIssues :many
-- Issues whose scheduled dispatch time has arrived and has not been consumed.
-- Used by the issue dispatch scheduler to enqueue deferred agent tasks.
SELECT * FROM issue
WHERE dispatch_after IS NOT NULL
  AND dispatch_after <= now()
  AND dispatch_after_fired_at IS NULL
  AND assignee_type = 'agent'
  AND assignee_id IS NOT NULL
  AND status NOT IN ('done', 'cancelled')
ORDER BY dispatch_after ASC
LIMIT $1;

-- name: ClaimIssueDispatch :one
-- Compare-and-set claim of a due dispatch. Returns no rows when another
-- scheduler instance already claimed it or the schedule changed.
UPDATE issue SET
    dispatch_after_fired_at = now()
WHERE id = $1
  AND dispatch_after IS NOT NULL
  AND dispatch_after <= now()
  AND dispatch_after_fired_at IS NULL
RETURNING *;

-- name: ResetIssueDispatchFired :exec
-- Releases a claimed dispatch so the scheduler retries on the next tick
-- (e.g. when no runtime was available at fire time).
UPDATE issue SET
    dispatch_after_fired_at = NULL
WHERE id = $1;

-- name: ResetIssueAgentMentionChain :exec
-- Resets the chain count and bumps the generation so any in-flight rollbacks
-- from the pre-reset epoch become no-ops.
UPDATE issue SET
    agent_mention_chain_count = 0,
    agent_mention_chain_generation = agent_mention_chain_generation + 1
WHERE id = $1;
