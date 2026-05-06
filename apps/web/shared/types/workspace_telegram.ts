export interface WorkspaceTelegramSettings {
  configured: boolean;
  chat_id?: string;
  enabled?: boolean;
}

export interface UpsertWorkspaceTelegramRequest {
  chat_id: string;
  enabled?: boolean;
}
