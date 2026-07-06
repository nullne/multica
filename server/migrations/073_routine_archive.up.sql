-- Add archive support to routines (soft-delete replacement).
-- archived_at IS NOT NULL means the routine is hidden from normal reads and triggers.
ALTER TABLE routine ADD COLUMN archived_at TIMESTAMPTZ DEFAULT NULL;

CREATE INDEX idx_routine_workspace_active ON routine(workspace_id, created_at DESC)
    WHERE archived_at IS NULL;
