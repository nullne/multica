import type { TelegramNotificationPreferences } from "./telegram_preferences";

export interface WorkspaceTelegramSettings {
  configured: boolean;
  chat_id?: string;
  enabled?: boolean;
  preferences?: TelegramNotificationPreferences;
}

export interface UpsertWorkspaceTelegramRequest {
  chat_id: string;
  enabled?: boolean;
  preferences?: Partial<TelegramNotificationPreferences>;
}
