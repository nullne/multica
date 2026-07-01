ALTER TABLE user_notification_channel
ADD COLUMN preferences JSONB NOT NULL DEFAULT '{}'::jsonb;
