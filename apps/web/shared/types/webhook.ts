export type WebhookSourceType = "standard" | "oss-alert" | "github";
export type WebhookStatus = "active" | "paused";
export type WebhookActionType = "create_issue" | "comment_issue";

export interface Webhook {
  id: string;
  workspace_id: string;
  name: string;
  source_type: WebhookSourceType;
  token_prefix: string;
  status: WebhookStatus;
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
  action_type: WebhookActionType;
  config:
    | CreateIssueActionConfig
    | CommentIssueActionConfig
    | Record<string, unknown>;
  enabled: boolean;
  position: number;
  created_at: string;
  updated_at: string;
}

export interface CreateIssueActionConfig {
  agent_id: string;
  title_template: string;
  description_template: string;
  labels: string[];
  dispatch_provider?: string;
  dispatch_daemon_id?: string;
  dispatch_daemon_label?: string;
  event_types?: string[];
  repos?: string[];
  subscriber_ids?: string[];
}

export interface CommentIssueActionConfig {
  content_template: string;
  mention_agent_id?: string;
  event_types?: string[];
  repos?: string[];
}

export interface WebhookWithActions {
  webhook: Webhook;
  event_count: number;
  actions: WebhookAction[];
}

export interface CreateWebhookRequest {
  name: string;
  source_type?: WebhookSourceType;
  dedup_window_seconds?: number;
  bot_user_id?: string;
  agent_id: string;
  title_template?: string;
  description_template?: string;
  labels?: string[];
  dispatch_provider?: string;
  dispatch_daemon_id?: string;
  dispatch_daemon_label?: string;
  subscriber_ids?: string[];
}

export interface CreateWebhookResponse {
  webhook: Webhook;
  token: string;
  actions: WebhookAction[];
}

export interface UpdateWebhookRequest {
  name?: string;
  source_type?: WebhookSourceType;
  status?: WebhookStatus;
  dedup_window_seconds?: number;
  bot_user_id?: string;
}

export interface CreateWebhookActionRequest {
  action_type: WebhookActionType;
  config:
    | CreateIssueActionConfig
    | CommentIssueActionConfig
    | Record<string, unknown>;
  enabled?: boolean;
  position?: number;
}

export interface UpdateWebhookActionRequest {
  action_type?: WebhookActionType;
  config?:
    | CreateIssueActionConfig
    | CommentIssueActionConfig
    | Record<string, unknown>;
  enabled?: boolean;
  position?: number;
}

export type WebhookEventStatus = "processed" | "filtered" | "deduped" | "error";

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

export interface AdapterKey {
  key: string;
  description: string;
  required: boolean;
}

export interface AdapterInfo {
  source_type: string;
  description: string;
  keys: AdapterKey[];
  example: string;
}

// Bot users are non-human members used as the author of webhook-driven
// comments. They appear in member lists like normal users but cannot log in.
//
// webhook_count is how many webhooks currently bind to this bot via
// bot_user_id; the Members tab uses it to warn before deletion.
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
