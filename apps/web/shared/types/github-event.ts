export type GitHubEventType = "push" | "pull_request" | "issues" | "issue_comment";

export interface GitHubEventRule {
  id: string;
  workspace_id: string;
  event_type: GitHubEventType;
  agent_id: string;
  enabled: boolean;
  title_template: string;
  description_template: string;
  dispatch_provider?: string | null;
  dispatch_daemon_id?: string | null;
  dispatch_daemon_label?: string | null;
  created_at: string;
  updated_at: string;
}

export interface UpsertGitHubEventRuleRequest {
  event_type: GitHubEventType;
  agent_id: string;
  enabled?: boolean;
  title_template?: string;
  description_template?: string;
  dispatch_provider?: string;
  dispatch_daemon_id?: string;
  dispatch_daemon_label?: string;
}
