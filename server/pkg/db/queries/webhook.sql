-- name: CreateWebhook :one
INSERT INTO webhook (workspace_id, name, source_type, token_hash, token_prefix, dedup_window_seconds, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetWebhook :one
SELECT * FROM webhook WHERE id = $1;

-- name: GetWebhookByTokenHash :one
SELECT * FROM webhook WHERE token_hash = $1;

-- name: ListWebhooksByWorkspace :many
SELECT * FROM webhook WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: UpdateWebhook :one
UPDATE webhook SET
    name = COALESCE(sqlc.narg('name'), name),
    source_type = COALESCE(sqlc.narg('source_type'), source_type),
    status = COALESCE(sqlc.narg('status'), status),
    dedup_window_seconds = COALESCE(sqlc.narg('dedup_window_seconds'), dedup_window_seconds),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteWebhook :exec
DELETE FROM webhook WHERE id = $1;

-- name: RegenerateWebhookToken :one
UPDATE webhook SET token_hash = $2, token_prefix = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateWebhookAction :one
INSERT INTO webhook_action (webhook_id, action_type, config, enabled, position)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetWebhookAction :one
SELECT * FROM webhook_action WHERE id = $1;

-- name: ListWebhookActions :many
SELECT * FROM webhook_action WHERE webhook_id = $1 ORDER BY position ASC;

-- name: ListEnabledWebhookActions :many
SELECT * FROM webhook_action WHERE webhook_id = $1 AND enabled = true ORDER BY position ASC;

-- name: UpdateWebhookAction :one
UPDATE webhook_action SET
    action_type = COALESCE(sqlc.narg('action_type'), action_type),
    config = COALESCE(sqlc.narg('config'), config),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    position = COALESCE(sqlc.narg('position'), position),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteWebhookAction :exec
DELETE FROM webhook_action WHERE id = $1;

-- name: CreateWebhookEventLog :one
INSERT INTO webhook_event_log (webhook_id, dedup_key, payload, status, issue_id, error_message)
VALUES ($1, $2, $3, $4, sqlc.narg(issue_id), sqlc.narg(error_message))
RETURNING *;

-- name: ListWebhookEvents :many
SELECT * FROM webhook_event_log
WHERE webhook_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountWebhookEvents :one
SELECT count(*) FROM webhook_event_log WHERE webhook_id = $1;

-- name: FindRecentWebhookEvent :one
SELECT id FROM webhook_event_log
WHERE webhook_id = $1 AND dedup_key = $2 AND status = 'processed'
  AND created_at > now() - make_interval(secs => @window_seconds::double precision)
LIMIT 1;
