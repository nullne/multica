-- name: CreateRecurringTemplate :one
INSERT INTO recurring_issue_template (
    workspace_id, title, description, priority,
    assignee_type, assignee_id, due_date_offset_hours,
    dispatch_provider, dispatch_daemon_id, dispatch_daemon_label,
    schedule, timezone, enabled, next_run_at, max_runs,
    created_by_id, created_by_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
)
RETURNING *;

-- name: GetRecurringTemplateInWorkspace :one
SELECT * FROM recurring_issue_template
WHERE id = $1 AND workspace_id = $2;

-- name: ListRecurringTemplates :many
SELECT * FROM recurring_issue_template
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: ListActiveRecurringTemplates :many
SELECT * FROM recurring_issue_template
WHERE workspace_id = $1
  AND enabled = TRUE
  AND (max_runs IS NULL OR successful_runs_count < max_runs)
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
    max_runs              = $15,
    updated_at            = now()
WHERE id = $1 AND workspace_id = $16
RETURNING *;

-- name: DeleteRecurringTemplate :exec
DELETE FROM recurring_issue_template WHERE id = $1 AND workspace_id = $2;

-- name: ListDueRecurringTemplates :many
SELECT * FROM recurring_issue_template
WHERE enabled = TRUE
  AND next_run_at IS NOT NULL
  AND next_run_at <= now()
  AND (max_runs IS NULL OR successful_runs_count < max_runs)
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
  AND (max_runs IS NULL OR successful_runs_count < max_runs)
RETURNING *;

-- name: IncrementRecurringTemplateSuccess :one
UPDATE recurring_issue_template
SET successful_runs_count = successful_runs_count + 1,
    next_run_at = CASE
        WHEN max_runs IS NOT NULL AND successful_runs_count + 1 >= max_runs THEN NULL
        ELSE next_run_at
    END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListRecurringTemplateSubscribers :many
SELECT * FROM recurring_template_subscriber
WHERE template_id = $1
ORDER BY created_at;

-- name: AddRecurringTemplateSubscriber :exec
INSERT INTO recurring_template_subscriber (template_id, user_id)
VALUES ($1, $2)
ON CONFLICT (template_id, user_id) DO NOTHING;

-- name: RemoveRecurringTemplateSubscriber :exec
DELETE FROM recurring_template_subscriber
WHERE template_id = $1 AND user_id = $2;

-- name: ClearRecurringTemplateSubscribers :exec
DELETE FROM recurring_template_subscriber
WHERE template_id = $1;
