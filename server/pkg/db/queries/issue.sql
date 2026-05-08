-- name: ListIssues :many
SELECT * FROM issue
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority'))
  AND (sqlc.narg('assignee_id')::uuid IS NULL OR assignee_id = sqlc.narg('assignee_id'))
ORDER BY position ASC, created_at DESC
LIMIT $2 OFFSET $3;

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
    dispatch_provider, dispatch_daemon_id, dispatch_daemon_label
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
    $16, $17, $18
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
-- issue. Returns the new count when the reservation succeeds. Returns no rows
-- (pgx.ErrNoRows) when the chain has already reached the supplied limit, so
-- concurrent callers cannot exceed the cap.
UPDATE issue SET
    agent_mention_chain_count = agent_mention_chain_count + 1
WHERE id = $1 AND agent_mention_chain_count < sqlc.arg(max_count)::int
RETURNING agent_mention_chain_count;

-- name: ReleaseIssueAgentMentionChainSlot :one
-- Releases a previously-reserved chain slot when the dispatch that consumed it
-- did not actually complete. GREATEST keeps the count at or above zero even if
-- a reset raced with the rollback.
UPDATE issue SET
    agent_mention_chain_count = GREATEST(agent_mention_chain_count - 1, 0)
WHERE id = $1
RETURNING agent_mention_chain_count;

-- name: ResetIssueAgentMentionChain :exec
UPDATE issue SET
    agent_mention_chain_count = 0
WHERE id = $1;
