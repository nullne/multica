CREATE TABLE recurring_issue_template (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    title                 TEXT NOT NULL,
    description           TEXT,
    priority              TEXT NOT NULL DEFAULT 'medium'
                              CHECK (priority IN ('urgent', 'high', 'medium', 'low', 'none')),
    assignee_type         TEXT CHECK (assignee_type IN ('member', 'agent')),
    assignee_id           UUID,
    due_date_offset_hours INTEGER,
    dispatch_provider     TEXT,
    dispatch_daemon_id    UUID,
    dispatch_daemon_label TEXT,
    schedule              TEXT NOT NULL,
    timezone              TEXT NOT NULL DEFAULT 'UTC',
    enabled               BOOLEAN NOT NULL DEFAULT TRUE,
    last_triggered_at     TIMESTAMPTZ,
    next_run_at           TIMESTAMPTZ,
    created_by_id         UUID NOT NULL,
    created_by_type       TEXT NOT NULL DEFAULT 'member'
                              CHECK (created_by_type IN ('member', 'agent')),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_recurring_template_workspace ON recurring_issue_template(workspace_id);
CREATE INDEX idx_recurring_template_due ON recurring_issue_template(next_run_at)
    WHERE enabled = TRUE AND next_run_at IS NOT NULL;
