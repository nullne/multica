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

// Adapter type that produced a webhook's events.
export type WebhookSourceType = "standard" | "oss-alert" | "github";

// Status of a single received webhook event.
export type WebhookEventStatus = "processed" | "filtered" | "deduped" | "error";

// A configured webhook endpoint in the workspace.
export interface Webhook {
  id: string;
  workspace_id: string;
  name: string;
  source_type: WebhookSourceType;
  token_prefix: string;
  status: "active" | "paused";
  dedup_window_seconds: number;
  bot_user_id: string | null;
  installation_id: number | null;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface WebhookAction {
  id: string;
  webhook_id: string;
  action_type: string;
  config: unknown;
  enabled: boolean;
  position: number;
  created_at: string;
  updated_at: string;
}

// One entry of the webhook list endpoint (GET /api/webhooks).
export interface WebhookListItem {
  webhook: Webhook;
  event_count: number;
  actions: WebhookAction[];
}

// A single received webhook event from the audit log
// (GET /api/webhooks/events).
export interface WebhookEvent {
  id: string;
  webhook_id: string;
  dedup_key: string;
  payload: unknown;
  status: WebhookEventStatus;
  issue_id: string | null;
  error_message: string | null;
  created_at: string;
}
