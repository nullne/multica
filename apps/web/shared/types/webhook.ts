export type WebhookSourceType = "standard" | "oss-alert";
export type WebhookStatus = "active" | "paused";

export interface Webhook {
  id: string;
  workspace_id: string;
  name: string;
  source_type: WebhookSourceType;
  token_prefix: string;
  status: WebhookStatus;
  dedup_window_seconds: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface WebhookAction {
  id: string;
  webhook_id: string;
  action_type: string;
  config: CreateIssueActionConfig | Record<string, unknown>;
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
  agent_id: string;
  title_template?: string;
  description_template?: string;
  labels?: string[];
  dispatch_provider?: string;
  dispatch_daemon_id?: string;
  dispatch_daemon_label?: string;
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
}

export interface UpdateWebhookActionRequest {
  action_type?: string;
  config?: CreateIssueActionConfig | Record<string, unknown>;
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
