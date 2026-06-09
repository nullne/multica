-- name: CreateRoutine :one
INSERT INTO routine (
    workspace_id, name, instructions, priority,
    assignee_type, assignee_id, due_date_offset_hours,
    dispatch_provider, dispatch_daemon_id, dispatch_daemon_label,
    enabled, github_auto_fix_enabled, created_by_id, created_by_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: CreateManagedRoutine :one
INSERT INTO routine (
    workspace_id, name, priority, enabled, managed, created_by_id, created_by_type
) VALUES (
    $1, $2, 'medium', TRUE, TRUE, $3, $4
)
RETURNING *;

-- name: GetManagedRoutineByWorkspace :one
SELECT * FROM routine
WHERE workspace_id = $1 AND managed = TRUE
ORDER BY created_at ASC
LIMIT 1;

-- name: DeleteManagedRoutineByWorkspace :exec
DELETE FROM routine
WHERE workspace_id = $1 AND managed = TRUE;

-- name: GetRoutineInWorkspace :one
SELECT * FROM routine
WHERE id = $1 AND workspace_id = $2;

-- name: GetRoutine :one
SELECT * FROM routine
WHERE id = $1;

-- name: ListRoutines :many
SELECT * FROM routine
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: UpdateRoutine :one
UPDATE routine SET
    name                  = $2,
    instructions          = $3,
    priority              = $4,
    assignee_type         = $5,
    assignee_id           = $6,
    due_date_offset_hours = $7,
    dispatch_provider     = $8,
    dispatch_daemon_id    = $9,
    dispatch_daemon_label = $10,
    enabled               = $11,
    github_auto_fix_enabled = $12,
    updated_at            = now()
WHERE id = $1 AND workspace_id = $13
RETURNING *;

-- name: DeleteRoutine :exec
DELETE FROM routine
WHERE id = $1 AND workspace_id = $2;

-- name: CreateRoutineTrigger :one
INSERT INTO routine_trigger (
    routine_id, trigger_type, source_type, token_hash, token_prefix,
    installation_id, schedule, timezone, run_at, next_run_at,
    dedup_window_seconds, max_runs, config, enabled
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: CreateRoutineTriggerWithID :one
INSERT INTO routine_trigger (
    id, routine_id, trigger_type, source_type, token_hash, token_prefix,
    installation_id, schedule, timezone, run_at, next_run_at,
    dedup_window_seconds, max_runs, config, enabled
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING *;

-- name: GetRoutineTrigger :one
SELECT * FROM routine_trigger
WHERE id = $1;

-- name: GetRoutineTriggerInWorkspace :one
SELECT rt.*
FROM routine_trigger rt
JOIN routine r ON r.id = rt.routine_id
WHERE rt.id = $1 AND r.workspace_id = $2;

-- name: GetRoutineTriggerByTokenHash :one
SELECT * FROM routine_trigger
WHERE token_hash = $1;

-- name: ListRoutineTriggers :many
SELECT * FROM routine_trigger
WHERE routine_id = $1
ORDER BY created_at;

-- name: ListRoutineTriggersByTypeAndWorkspace :many
SELECT rt.*
FROM routine_trigger rt
JOIN routine r ON r.id = rt.routine_id
WHERE r.workspace_id = $1
  AND rt.trigger_type = $2
  AND r.enabled = TRUE
  AND rt.enabled = TRUE
ORDER BY rt.created_at;

-- name: ListRoutineTriggersByInstallationID :many
SELECT rt.*
FROM routine_trigger rt
JOIN routine r ON r.id = rt.routine_id
WHERE rt.trigger_type = 'github'
  AND rt.installation_id = $1
  AND r.enabled = TRUE
  AND rt.enabled = TRUE
ORDER BY rt.created_at;

-- name: UpdateRoutineTrigger :one
UPDATE routine_trigger SET
    source_type           = $2,
    token_hash            = $3,
    token_prefix          = $4,
    installation_id       = $5,
    schedule              = $6,
    timezone              = $7,
    run_at                = $8,
    next_run_at           = $9,
    dedup_window_seconds  = $10,
    max_runs              = $11,
    config                = $12,
    enabled               = $13,
    updated_at            = now()
WHERE id = $1
RETURNING *;

-- name: DeleteRoutineTrigger :exec
DELETE FROM routine_trigger
WHERE id = $1;

-- name: ListDueRoutineScheduleTriggers :many
SELECT rt.*
FROM routine_trigger rt
JOIN routine r ON r.id = rt.routine_id
WHERE r.enabled = TRUE
  AND rt.enabled = TRUE
  AND rt.trigger_type = 'schedule'
  AND rt.next_run_at IS NOT NULL
  AND rt.next_run_at <= now()
  AND (rt.max_runs IS NULL OR rt.successful_runs_count < rt.max_runs)
ORDER BY rt.next_run_at
LIMIT $1;

-- name: ClaimRoutineScheduleTrigger :one
UPDATE routine_trigger
SET last_triggered_at = now(),
    next_run_at       = $3,
    updated_at        = now()
WHERE id = $1
  AND next_run_at = $2
  AND trigger_type = 'schedule'
  AND enabled = TRUE
  AND (max_runs IS NULL OR successful_runs_count < max_runs)
RETURNING *;

-- name: IncrementRoutineTriggerSuccess :one
UPDATE routine_trigger
SET successful_runs_count = successful_runs_count + 1,
    next_run_at = CASE
        WHEN max_runs IS NOT NULL AND successful_runs_count + 1 >= max_runs THEN NULL
        ELSE next_run_at
    END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateRoutineAction :one
INSERT INTO routine_action (routine_id, action_type, config, enabled, position)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRoutineAction :one
SELECT * FROM routine_action
WHERE id = $1;

-- name: ListRoutineActions :many
SELECT * FROM routine_action
WHERE routine_id = $1
ORDER BY position ASC;

-- name: ListEnabledRoutineActions :many
SELECT * FROM routine_action
WHERE routine_id = $1 AND enabled = TRUE
ORDER BY position ASC;

-- name: UpdateRoutineAction :one
UPDATE routine_action SET
    action_type = $2,
    config      = $3,
    enabled     = $4,
    position    = $5,
    updated_at  = now()
WHERE id = $1
RETURNING *;

-- name: DeleteRoutineAction :exec
DELETE FROM routine_action
WHERE id = $1;

-- name: ListRoutineSubscribers :many
SELECT * FROM routine_subscriber
WHERE routine_id = $1
ORDER BY created_at;

-- name: AddRoutineSubscriber :exec
INSERT INTO routine_subscriber (routine_id, user_id)
VALUES ($1, $2)
ON CONFLICT (routine_id, user_id) DO NOTHING;

-- name: ClearRoutineSubscribers :exec
DELETE FROM routine_subscriber
WHERE routine_id = $1;

-- name: ListRoutineLabels :many
SELECT * FROM routine_label
WHERE routine_id = $1
ORDER BY created_at;

-- name: AddRoutineLabel :exec
INSERT INTO routine_label (routine_id, label_id)
VALUES ($1, $2)
ON CONFLICT (routine_id, label_id) DO NOTHING;

-- name: ClearRoutineLabels :exec
DELETE FROM routine_label
WHERE routine_id = $1;

-- name: CreateRoutineRun :one
INSERT INTO routine_run (
    routine_id, trigger_id, action_id, event_type, dedup_key,
    payload, status, issue_id, comment_id, error_message
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: ListRoutineRuns :many
SELECT * FROM routine_run
WHERE routine_id = $1
  AND (
    sqlc.arg(source)::text = ''
    OR (sqlc.arg(source)::text = 'scheduled' AND event_type = 'schedule')
    OR (sqlc.arg(source)::text = 'manual' AND event_type = 'manual')
    OR (sqlc.arg(source)::text = 'webhook' AND (event_type LIKE 'github.%' OR event_type LIKE 'alert.%'))
    OR (sqlc.arg(source)::text = 'api' AND event_type <> 'schedule' AND event_type <> 'manual' AND event_type NOT LIKE 'github.%' AND event_type NOT LIKE 'alert.%')
  )
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetRoutineOriginForIssue :one
SELECT
    rr.issue_id,
    rr.id AS run_id,
    rr.routine_id,
    r.name AS routine_name,
    rr.trigger_id,
    rt.trigger_type,
    rt.source_type,
    rt.token_prefix,
    rt.schedule,
    rt.timezone,
    rt.run_at,
    rr.action_id,
    ra.action_type,
    rr.event_type,
    rr.created_at
FROM routine_run rr
JOIN routine r ON r.id = rr.routine_id
LEFT JOIN routine_trigger rt ON rt.id = rr.trigger_id
LEFT JOIN routine_action ra ON ra.id = rr.action_id
WHERE rr.issue_id = $1
  AND r.workspace_id = $2
  AND rr.status = 'processed'
ORDER BY rr.created_at ASC
LIMIT 1;

-- name: ListRoutineOriginsByIssues :many
SELECT DISTINCT ON (rr.issue_id)
    rr.issue_id,
    rr.id AS run_id,
    rr.routine_id,
    r.name AS routine_name,
    rr.trigger_id,
    rt.trigger_type,
    rt.source_type,
    rt.token_prefix,
    rt.schedule,
    rt.timezone,
    rt.run_at,
    rr.action_id,
    ra.action_type,
    rr.event_type,
    rr.created_at
FROM routine_run rr
JOIN routine r ON r.id = rr.routine_id
LEFT JOIN routine_trigger rt ON rt.id = rr.trigger_id
LEFT JOIN routine_action ra ON ra.id = rr.action_id
WHERE rr.issue_id = ANY(sqlc.arg('issue_ids')::uuid[])
  AND r.workspace_id = sqlc.arg('workspace_id')
  AND rr.status = 'processed'
ORDER BY rr.issue_id, rr.created_at ASC;

-- name: GetRoutineIssueLinkByURL :one
SELECT il.*
FROM issue_link il
JOIN routine_run rr ON rr.issue_id = il.issue_id
WHERE rr.routine_id = $1
  AND il.workspace_id = $2
  AND il.url = $3
  AND rr.status = 'processed'
  AND rr.id = (
    SELECT rr_origin.id
    FROM routine_run rr_origin
    WHERE rr_origin.issue_id = il.issue_id
      AND rr_origin.status = 'processed'
    ORDER BY rr_origin.created_at ASC
    LIMIT 1
  )
ORDER BY rr.created_at ASC
LIMIT 1;

-- name: FindRecentRoutineRun :one
SELECT id FROM routine_run
WHERE trigger_id = $1
  AND dedup_key = $2
  AND status = 'processed'
  AND created_at > now() - make_interval(secs => @window_seconds::double precision)
LIMIT 1;

-- name: CreateRoutineTriggerTokenDraft :one
INSERT INTO routine_trigger_token_draft (workspace_id, created_by_id, token_hash, token_prefix)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ConsumeRoutineTriggerTokenDraft :one
UPDATE routine_trigger_token_draft
SET consumed_at = now()
WHERE id = $1
  AND workspace_id = $2
  AND consumed_at IS NULL
  AND expires_at > now()
RETURNING *;
