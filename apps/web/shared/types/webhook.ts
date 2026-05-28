// Bot users are non-human members used as automation authors. They appear in
// member lists like normal users but cannot log in.
export interface BotUser {
  id: string;
  name: string;
  email: string;
  avatar_url: string | null;
  kind: "bot";
  webhook_count: number;
  created_at: string;
}

export interface CreateBotUserRequest {
  name: string;
  avatar_url?: string;
}
