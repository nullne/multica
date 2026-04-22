export type GitHubEventType = "push" | "pull_request" | "issues" | "issue_comment";

export interface GitHubEventRule {
  id: string;
  workspace_id: string;
  event_type: GitHubEventType;
  /**
   * Repository this rule applies to in "owner/repo" form. Empty string means
   * the rule is the workspace-wide default for the event type and is used as
   * a fallback when no per-repo rule matches.
   */
  repo_full_name: string;
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
  /** Empty / omitted = workspace-default rule for the event type. */
  repo_full_name?: string;
  agent_id: string;
  enabled?: boolean;
  title_template?: string;
  description_template?: string;
  dispatch_provider?: string;
  dispatch_daemon_id?: string;
  dispatch_daemon_label?: string;
}
