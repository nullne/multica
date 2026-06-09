CREATE TABLE routine_event (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id         UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    source_type          TEXT NOT NULL,
    event_type           TEXT NOT NULL DEFAULT '',
    dedup_key            TEXT NOT NULL DEFAULT '',
    external_delivery_id TEXT,
    data                 JSONB NOT NULL DEFAULT '{}',
    payload              JSONB NOT NULL DEFAULT '{}',
    status               TEXT NOT NULL CHECK (status IN (
        'received',
        'processed',
        'filtered',
        'deduped',
        'no_matching_trigger',
        'parse_error',
        'error'
    )),
    error_message        TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_routine_event_workspace_created ON routine_event(workspace_id, created_at DESC);
CREATE INDEX idx_routine_event_workspace_source ON routine_event(workspace_id, source_type, event_type, created_at DESC);
CREATE INDEX idx_routine_event_delivery ON routine_event(source_type, external_delivery_id)
    WHERE external_delivery_id IS NOT NULL;

ALTER TABLE routine_run
    ADD COLUMN routine_event_id UUID REFERENCES routine_event(id) ON DELETE SET NULL;

CREATE INDEX idx_routine_run_routine_event ON routine_run(routine_event_id)
    WHERE routine_event_id IS NOT NULL;
