-- Best-effort recreation of the legacy tables for local rollback. The
-- product was not live when these were removed, so historical rows are not
-- restored.
CREATE TABLE recurring_issue_template (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    title                 TEXT NOT NULL,
    description           TEXT,
    priority              TEXT NOT NULL DEFAULT 'medium',
    assignee_type         TEXT,
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
    created_by_type       TEXT NOT NULL DEFAULT 'member',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    max_runs              INTEGER,
    successful_runs_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE recurring_template_subscriber (
    template_id UUID NOT NULL REFERENCES recurring_issue_template(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (template_id, user_id)
);

CREATE TABLE webhook (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id         UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name                 TEXT NOT NULL,
    source_type          TEXT NOT NULL DEFAULT 'standard',
    token_hash           TEXT NOT NULL UNIQUE,
    token_prefix         TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused')),
    dedup_window_seconds INT NOT NULL DEFAULT 600,
    created_by           UUID NOT NULL REFERENCES "user"(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    bot_user_id          UUID,
    installation_id      BIGINT
);

CREATE INDEX idx_webhook_workspace ON webhook(workspace_id);
CREATE INDEX idx_webhook_token_hash ON webhook(token_hash);

CREATE TABLE webhook_action (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id  UUID NOT NULL REFERENCES webhook(id) ON DELETE CASCADE,
    action_type TEXT NOT NULL DEFAULT 'create_issue',
    config      JSONB NOT NULL DEFAULT '{}',
    enabled     BOOLEAN NOT NULL DEFAULT true,
    position    INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_action_webhook ON webhook_action(webhook_id, position);

CREATE TABLE webhook_event_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id    UUID NOT NULL REFERENCES webhook(id) ON DELETE CASCADE,
    dedup_key     TEXT NOT NULL DEFAULT '',
    payload       JSONB NOT NULL,
    status        TEXT NOT NULL DEFAULT 'processed' CHECK (status IN ('processed', 'filtered', 'deduped', 'error')),
    issue_id      UUID REFERENCES issue(id) ON DELETE SET NULL,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_event_log_webhook ON webhook_event_log(webhook_id, created_at DESC);
CREATE INDEX idx_webhook_event_log_dedup ON webhook_event_log(webhook_id, dedup_key, created_at DESC);

DROP INDEX IF EXISTS idx_routine_managed_workspace;
ALTER TABLE routine DROP COLUMN IF EXISTS managed;
