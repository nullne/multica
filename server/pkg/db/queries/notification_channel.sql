-- name: GetUserNotificationChannel :one
SELECT * FROM user_notification_channel
WHERE user_id = $1 AND channel_type = $2;

-- name: ListUserNotificationChannels :many
SELECT * FROM user_notification_channel
WHERE user_id = $1
ORDER BY channel_type;

-- name: UpsertUserNotificationChannel :one
INSERT INTO user_notification_channel (user_id, channel_type, channel_id, enabled)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, channel_type) DO UPDATE SET
    channel_id = EXCLUDED.channel_id,
    enabled    = EXCLUDED.enabled,
    updated_at = now()
RETURNING *;

-- name: DeleteUserNotificationChannel :exec
DELETE FROM user_notification_channel
WHERE user_id = $1 AND channel_type = $2;

-- name: ListEnabledTelegramChannelsForUsers :many
SELECT unc.user_id, unc.channel_id
FROM user_notification_channel unc
WHERE unc.channel_type = 'telegram'
  AND unc.enabled = TRUE
  AND unc.user_id = ANY($1::uuid[]);
