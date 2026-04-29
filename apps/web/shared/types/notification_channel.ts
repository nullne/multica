export type NotificationChannelType = "telegram";

export interface NotificationChannel {
  channel_type: NotificationChannelType;
  channel_id: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface UpsertTelegramChannelRequest {
  chat_id: string;
  enabled?: boolean;
}
