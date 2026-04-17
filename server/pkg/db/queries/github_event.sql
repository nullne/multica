-- name: GetWorkspaceByInstallationID :one
SELECT * FROM workspace WHERE github_installation_id = $1;

-- name: GetGitHubEventRule :one
-- Returns the most-specific enabled rule for the given event:
--   1. Exact match on (workspace_id, event_type, repo_full_name).
--   2. Falls back to the workspace-default rule (repo_full_name = '').
SELECT * FROM github_event_rule
WHERE workspace_id = $1
  AND event_type = $2
  AND repo_full_name IN ($3, '')
ORDER BY (repo_full_name = '') ASC
LIMIT 1;

-- name: ListGitHubEventRules :many
SELECT * FROM github_event_rule
WHERE workspace_id = $1
ORDER BY event_type ASC, repo_full_name ASC;

-- name: UpsertGitHubEventRule :one
INSERT INTO github_event_rule (workspace_id, event_type, repo_full_name, agent_id, enabled, title_template, description_template, dispatch_provider, dispatch_daemon_id, dispatch_daemon_label)
VALUES ($1, $2, $3, $4, $5, $6, $7, sqlc.narg(dispatch_provider), sqlc.narg(dispatch_daemon_id), sqlc.narg(dispatch_daemon_label))
ON CONFLICT (workspace_id, event_type, repo_full_name) DO UPDATE SET
    agent_id = EXCLUDED.agent_id,
    enabled = EXCLUDED.enabled,
    title_template = EXCLUDED.title_template,
    description_template = EXCLUDED.description_template,
    dispatch_provider = COALESCE(EXCLUDED.dispatch_provider, github_event_rule.dispatch_provider),
    dispatch_daemon_id = EXCLUDED.dispatch_daemon_id,
    dispatch_daemon_label = EXCLUDED.dispatch_daemon_label,
    updated_at = now()
RETURNING *;

-- name: DeleteGitHubEventRule :exec
DELETE FROM github_event_rule WHERE id = $1;
