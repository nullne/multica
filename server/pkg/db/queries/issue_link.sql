-- name: CreateIssueLink :one
-- Insert a new external link for an issue. Returns the existing row when the
-- (issue_id, url) pair is already linked (idempotent for repeated webhook
-- ingest of the same source URL).
INSERT INTO issue_link (issue_id, workspace_id, source_type, kind, direction, url, external_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (issue_id, url) DO UPDATE
    SET workspace_id = EXCLUDED.workspace_id,
        source_type = EXCLUDED.source_type,
        kind        = EXCLUDED.kind,
        direction   = EXCLUDED.direction,
        external_id = EXCLUDED.external_id
RETURNING *;

-- name: GetIssueLinkByURL :one
-- Reverse lookup used by webhook actions to find the Multica issue that
-- corresponds to an incoming external URL (PR, GitHub issue, etc.).
SELECT * FROM issue_link
WHERE workspace_id = $1 AND url = $2;

-- name: ListIssueLinksByURL :many
SELECT * FROM issue_link
WHERE workspace_id = $1 AND url = $2
ORDER BY created_at ASC;

-- name: ListIssueLinksByIssue :many
SELECT * FROM issue_link
WHERE issue_id = $1
ORDER BY created_at ASC;

-- name: ListIssueLinksByIssues :many
-- Batch fetch of links for multiple issues; used by issue listing endpoints.
SELECT * FROM issue_link
WHERE issue_id = ANY($1::uuid[])
ORDER BY created_at ASC;

-- name: DeleteIssueLink :exec
DELETE FROM issue_link WHERE id = $1;

-- name: DeleteIssueLinksByIssue :exec
DELETE FROM issue_link WHERE issue_id = $1;
