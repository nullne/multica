-- Webhooks: workspace-scoped endpoints for receiving external events
CREATE TABLE webhook (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    source_type     TEXT NOT NULL DEFAULT 'standard' CHECK (source_type IN ('standard', 'oss-alert')),
    token_hash      TEXT NOT NULL UNIQUE,
    token_prefix    TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused')),
    dedup_window_seconds INT NOT NULL DEFAULT 600,
    created_by      UUID NOT NULL REFERENCES "user"(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_workspace ON webhook(workspace_id);
CREATE INDEX idx_webhook_token_hash ON webhook(token_hash);

-- Webhook actions: what to do when an event is received
CREATE TABLE webhook_action (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id      UUID NOT NULL REFERENCES webhook(id) ON DELETE CASCADE,
    action_type     TEXT NOT NULL DEFAULT 'create_issue' CHECK (action_type IN ('create_issue')),
    config          JSONB NOT NULL DEFAULT '{}',
    enabled         BOOLEAN NOT NULL DEFAULT true,
    position        INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_action_webhook ON webhook_action(webhook_id, position);

-- Webhook event log: audit trail of every received webhook invocation
CREATE TABLE webhook_event_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id      UUID NOT NULL REFERENCES webhook(id) ON DELETE CASCADE,
    dedup_key       TEXT NOT NULL DEFAULT '',
    payload         JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'processed' CHECK (status IN ('processed', 'filtered', 'deduped', 'error')),
    issue_id        UUID REFERENCES issue(id) ON DELETE SET NULL,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_event_log_webhook ON webhook_event_log(webhook_id, created_at DESC);
CREATE INDEX idx_webhook_event_log_dedup ON webhook_event_log(webhook_id, dedup_key, created_at DESC);
