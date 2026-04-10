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
    max_verification_rounds
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
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
    position = COALESCE(sqlc.narg('position'), position),
    due_date = sqlc.narg('due_date'),
    max_verification_rounds = sqlc.narg('max_verification_rounds'),
    updated_at = now()
WHERE id = $1
RETURNING *;

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

-- name: UpdateIssueDevLinks :one
UPDATE issue SET
    linked_branch = COALESCE(sqlc.narg('linked_branch'), linked_branch),
    linked_pr_url = COALESCE(sqlc.narg('linked_pr_url'), linked_pr_url),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteIssue :exec
DELETE FROM issue WHERE id = $1;
