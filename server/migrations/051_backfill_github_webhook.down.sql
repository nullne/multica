-- Remove the backfilled GitHub webhook rows. Identifiable by their
-- token_prefix sentinel; user-managed github webhooks are created via the
-- /api/workspaces/:id/github/connect handler which generates a real random
-- token (40 hex chars after a "whk_" prefix; truncated to 12 chars).
DELETE FROM webhook
WHERE source_type = 'github'
  AND token_prefix = 'github-app-';
