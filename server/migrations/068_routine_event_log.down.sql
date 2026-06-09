DROP INDEX IF EXISTS idx_routine_run_routine_event;

ALTER TABLE routine_run
    DROP COLUMN IF EXISTS routine_event_id;

DROP INDEX IF EXISTS idx_routine_event_delivery;
DROP INDEX IF EXISTS idx_routine_event_workspace_source;
DROP INDEX IF EXISTS idx_routine_event_workspace_created;

DROP TABLE IF EXISTS routine_event;
