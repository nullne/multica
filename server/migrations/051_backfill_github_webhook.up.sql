-- Backfill source_type='github' webhook rows for workspaces that already
-- had a GitHub App installation before migration 050. Without this row
-- /api/github/events would silently drop incoming events because
-- GetWebhookByInstallationID would return ErrNoRows.
--
-- token_hash/token_prefix are required (NOT NULL UNIQUE) by the schema even
-- though GitHub webhooks authenticate by HMAC, so we synthesize a random
-- value per row. created_by is set to a workspace owner (any owner; falls
-- back to any member if no owner exists).

INSERT INTO webhook (
    workspace_id, name, source_type,
    token_hash, token_prefix,
    dedup_window_seconds, created_by, installation_id
)
SELECT
    w.id,
    'GitHub App',
    'github',
    encode(digest('github-backfill:' || w.id::text || ':' || w.github_installation_id::text, 'sha256'), 'hex'),
    'github-app-',
    600,
    (
        SELECT user_id
        FROM member
        WHERE workspace_id = w.id
        ORDER BY (role = 'owner') DESC, (role = 'admin') DESC, created_at ASC
        LIMIT 1
    ),
    w.github_installation_id
FROM workspace w
WHERE w.github_installation_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM webhook
      WHERE installation_id = w.github_installation_id
  );
