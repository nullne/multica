DROP INDEX IF EXISTS idx_github_event_rule_lookup;

-- Collapse multi-repo rules back to a single (workspace_id, event_type) row.
-- Keep the workspace-default rule (repo_full_name = '') if present, otherwise
-- the lexicographically first repo-specific rule.
DELETE FROM github_event_rule g
USING (
    SELECT id FROM (
        SELECT id,
               ROW_NUMBER() OVER (
                   PARTITION BY workspace_id, event_type
                   ORDER BY (repo_full_name = '') DESC, repo_full_name ASC, created_at ASC
               ) AS rn
        FROM github_event_rule
    ) t
    WHERE rn > 1
) dup
WHERE g.id = dup.id;

ALTER TABLE github_event_rule
    DROP CONSTRAINT github_event_rule_workspace_event_repo_key;

ALTER TABLE github_event_rule
    ADD CONSTRAINT github_event_rule_workspace_id_event_type_key
    UNIQUE (workspace_id, event_type);

ALTER TABLE github_event_rule DROP COLUMN repo_full_name;
