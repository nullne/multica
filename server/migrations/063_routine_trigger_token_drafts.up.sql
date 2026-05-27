CREATE TABLE routine_trigger_token_draft (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    created_by_id UUID NOT NULL,
    token_hash    TEXT NOT NULL UNIQUE,
    token_prefix  TEXT NOT NULL,
    consumed_at   TIMESTAMPTZ,
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT now() + interval '24 hours',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_routine_trigger_token_draft_active
    ON routine_trigger_token_draft(workspace_id, created_by_id, created_at DESC)
    WHERE consumed_at IS NULL;
