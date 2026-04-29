-- User external notification channels (e.g. Telegram)
CREATE TABLE user_notification_channel (
    user_id      UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    channel_type TEXT NOT NULL CHECK (channel_type IN ('telegram')),
    channel_id   TEXT NOT NULL,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, channel_type)
);

CREATE INDEX idx_user_notification_channel_user ON user_notification_channel(user_id);
