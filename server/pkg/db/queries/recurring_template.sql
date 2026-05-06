-- name: CreateRecurringTemplate :one
INSERT INTO recurring_issue_template (
    workspace_id, title, description, priority,
    assignee_type, assignee_id, due_date_offset_hours,
    dispatch_provider, dispatch_daemon_id, dispatch_daemon_label,
    schedule, timezone, enabled, next_run_at,
    created_by_id, created_by_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
)
RETURNING *;

-- name: GetRecurringTemplateInWorkspace :one
SELECT * FROM recurring_issue_template
WHERE id = $1 AND workspace_id = $2;

-- name: ListRecurringTemplates :many
SELECT * FROM recurring_issue_template
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: UpdateRecurringTemplate :one
UPDATE recurring_issue_template SET
    title                 = $2,
    description           = $3,
    priority              = $4,
    assignee_type         = $5,
    assignee_id           = $6,
    due_date_offset_hours = $7,
    dispatch_provider     = $8,
    dispatch_daemon_id    = $9,
    dispatch_daemon_label = $10,
    schedule              = $11,
    timezone              = $12,
    enabled               = $13,
    next_run_at           = $14,
    updated_at            = now()
WHERE id = $1 AND workspace_id = $15
RETURNING *;

-- name: DeleteRecurringTemplate :exec
DELETE FROM recurring_issue_template WHERE id = $1 AND workspace_id = $2;

-- name: ListDueRecurringTemplates :many
SELECT * FROM recurring_issue_template
WHERE enabled = TRUE
  AND next_run_at IS NOT NULL
  AND next_run_at <= now()
ORDER BY next_run_at
LIMIT $1;

-- name: ClaimRecurringTemplate :one
UPDATE recurring_issue_template
SET last_triggered_at = now(),
    next_run_at       = $3,
    updated_at        = now()
WHERE id = $1
  AND next_run_at = $2
  AND enabled = TRUE
RETURNING *;
