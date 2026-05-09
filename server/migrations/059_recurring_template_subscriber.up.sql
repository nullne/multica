-- Recurring template subscribers: members that should be auto-subscribed to
-- every issue produced by the template. Member-only for now, matching the
-- broader subscriber-notification model where only members receive
-- inbox/Telegram notifications.
CREATE TABLE recurring_template_subscriber (
    template_id UUID NOT NULL REFERENCES recurring_issue_template(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (template_id, user_id)
);

CREATE INDEX idx_recurring_template_subscriber_template
    ON recurring_template_subscriber(template_id);
