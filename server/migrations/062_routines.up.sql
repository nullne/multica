-- Unified routines: issue templates with schedule/API/GitHub triggers and actions.
CREATE TABLE routine (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    instructions          TEXT,
    priority              TEXT NOT NULL DEFAULT 'medium'
                              CHECK (priority IN ('urgent', 'high', 'medium', 'low', 'none')),
    assignee_type         TEXT CHECK (assignee_type IN ('member', 'agent')),
    assignee_id           UUID,
    due_date_offset_hours INTEGER,
    dispatch_provider     TEXT,
    dispatch_daemon_id    UUID,
    dispatch_daemon_label TEXT,
    enabled               BOOLEAN NOT NULL DEFAULT TRUE,
    created_by_id         UUID NOT NULL,
    created_by_type       TEXT NOT NULL DEFAULT 'member'
                              CHECK (created_by_type IN ('member', 'agent', 'webhook', 'github')),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_routine_workspace ON routine(workspace_id);

CREATE TABLE routine_trigger (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    routine_id            UUID NOT NULL REFERENCES routine(id) ON DELETE CASCADE,
    trigger_type          TEXT NOT NULL CHECK (trigger_type IN ('schedule', 'api', 'github')),
    source_type           TEXT,
    token_hash            TEXT UNIQUE,
    token_prefix          TEXT NOT NULL DEFAULT '',
    installation_id       BIGINT,
    schedule              TEXT,
    timezone              TEXT NOT NULL DEFAULT 'UTC',
    run_at                TIMESTAMPTZ,
    next_run_at           TIMESTAMPTZ,
    last_triggered_at     TIMESTAMPTZ,
    dedup_window_seconds  INTEGER NOT NULL DEFAULT 600,
    max_runs              INTEGER CHECK (max_runs IS NULL OR max_runs > 0),
    successful_runs_count INTEGER NOT NULL DEFAULT 0 CHECK (successful_runs_count >= 0),
    config                JSONB NOT NULL DEFAULT '{}',
    enabled               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_routine_trigger_routine ON routine_trigger(routine_id);
CREATE INDEX idx_routine_trigger_schedule_due ON routine_trigger(next_run_at)
    WHERE trigger_type = 'schedule'
      AND enabled = TRUE
      AND next_run_at IS NOT NULL
      AND (max_runs IS NULL OR successful_runs_count < max_runs);
CREATE INDEX idx_routine_trigger_installation_id ON routine_trigger(installation_id)
    WHERE installation_id IS NOT NULL;

CREATE TABLE routine_action (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    routine_id  UUID NOT NULL REFERENCES routine(id) ON DELETE CASCADE,
    action_type TEXT NOT NULL CHECK (action_type IN ('create_issue', 'comment_issue')),
    config      JSONB NOT NULL DEFAULT '{}',
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    position    INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_routine_action_routine ON routine_action(routine_id, position);

CREATE TABLE routine_subscriber (
    routine_id UUID NOT NULL REFERENCES routine(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (routine_id, user_id)
);

CREATE INDEX idx_routine_subscriber_routine ON routine_subscriber(routine_id);

CREATE TABLE routine_label (
    routine_id UUID NOT NULL REFERENCES routine(id) ON DELETE CASCADE,
    label_id   UUID NOT NULL REFERENCES issue_label(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (routine_id, label_id)
);

CREATE INDEX idx_routine_label_routine ON routine_label(routine_id);

CREATE TABLE routine_run (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    routine_id    UUID NOT NULL REFERENCES routine(id) ON DELETE CASCADE,
    trigger_id    UUID REFERENCES routine_trigger(id) ON DELETE SET NULL,
    action_id     UUID REFERENCES routine_action(id) ON DELETE SET NULL,
    event_type    TEXT NOT NULL DEFAULT '',
    dedup_key     TEXT NOT NULL DEFAULT '',
    payload       JSONB NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL CHECK (status IN ('processed', 'filtered', 'deduped', 'error')),
    issue_id      UUID REFERENCES issue(id) ON DELETE SET NULL,
    comment_id    UUID REFERENCES comment(id) ON DELETE SET NULL,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_routine_run_routine ON routine_run(routine_id, created_at DESC);
CREATE INDEX idx_routine_run_trigger_dedup ON routine_run(trigger_id, dedup_key, created_at DESC);

-- Backfill recurring templates into schedule routines.
INSERT INTO routine (
    id, workspace_id, name, instructions, priority,
    assignee_type, assignee_id, due_date_offset_hours,
    dispatch_provider, dispatch_daemon_id, dispatch_daemon_label,
    enabled, created_by_id, created_by_type, created_at, updated_at
)
SELECT
    id, workspace_id, title, description, priority,
    assignee_type, assignee_id, due_date_offset_hours,
    dispatch_provider, dispatch_daemon_id, dispatch_daemon_label,
    enabled, created_by_id, created_by_type, created_at, updated_at
FROM recurring_issue_template;

INSERT INTO routine_trigger (
    routine_id, trigger_type, schedule, timezone, next_run_at,
    last_triggered_at, max_runs, successful_runs_count, enabled, created_at, updated_at
)
SELECT
    id, 'schedule', schedule, timezone, next_run_at,
    last_triggered_at, max_runs, successful_runs_count, enabled, created_at, updated_at
FROM recurring_issue_template;

INSERT INTO routine_action (routine_id, action_type, config, enabled, position, created_at, updated_at)
SELECT id, 'create_issue', '{}'::jsonb, TRUE, 0, created_at, updated_at
FROM recurring_issue_template;

INSERT INTO routine_subscriber (routine_id, user_id, created_at)
SELECT template_id, user_id, created_at
FROM recurring_template_subscriber
ON CONFLICT DO NOTHING;

-- Backfill webhooks into event-triggered routines. Action-specific issue template
-- details remain in routine_action.config because webhook actions already store
-- event templates, labels, dispatch and subscribers there.
INSERT INTO routine (
    id, workspace_id, name, instructions, priority,
    enabled, created_by_id, created_by_type, created_at, updated_at
)
SELECT
    id, workspace_id, name, NULL, 'medium',
    status = 'active', created_by,
    CASE WHEN source_type = 'github' THEN 'github' ELSE 'webhook' END,
    created_at, updated_at
FROM webhook
ON CONFLICT (id) DO NOTHING;

INSERT INTO routine_trigger (
    routine_id, trigger_type, source_type, token_hash, token_prefix,
    installation_id, dedup_window_seconds, enabled, created_at, updated_at
)
SELECT
    id,
    CASE WHEN source_type = 'github' THEN 'github' ELSE 'api' END,
    source_type,
    CASE WHEN source_type = 'github' THEN NULL ELSE token_hash END,
    token_prefix,
    installation_id,
    dedup_window_seconds,
    status = 'active',
    created_at,
    updated_at
FROM webhook;

INSERT INTO routine_action (routine_id, action_type, config, enabled, position, created_at, updated_at)
SELECT webhook_id, action_type, config, enabled, position, created_at, updated_at
FROM webhook_action;

-- Disable legacy automation sources after backfill so routines are the only
-- active execution path.
UPDATE recurring_issue_template
SET enabled = FALSE,
    updated_at = now()
WHERE enabled = TRUE;

UPDATE webhook
SET status = 'paused',
    updated_at = now()
WHERE status = 'active';
