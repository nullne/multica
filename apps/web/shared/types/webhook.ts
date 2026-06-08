// Bot users are non-human members used as automation authors (e.g. the GitHub
// auto-fix routine). They appear in member lists like normal users but cannot
// log in.
export interface BotUser {
  id: string;
  name: string;
  email: string;
  avatar_url: string | null;
  kind: "bot";
  created_at: string;
}

export interface CreateBotUserRequest {
  name: string;
  avatar_url?: string;
}
