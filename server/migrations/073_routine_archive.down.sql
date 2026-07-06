DROP INDEX IF EXISTS idx_routine_workspace_active;

ALTER TABLE routine DROP COLUMN IF EXISTS archived_at;
