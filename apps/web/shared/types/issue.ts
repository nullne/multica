export type IssueStatus =
  | "backlog"
  | "todo"
  | "in_progress"
  | "in_review"
  | "done"
  | "blocked"
  | "cancelled";

export type IssuePriority = "urgent" | "high" | "medium" | "low" | "none";

export type IssueAssigneeType = "member" | "agent";
export type IssueCreatorType = "member" | "agent" | "webhook" | "github";

export interface IssueReaction {
  id: string;
  issue_id: string;
  actor_type: string;
  actor_id: string;
  emoji: string;
  created_at: string;
}

export interface AcceptanceCriterion {
  id: string;
  description: string;
  [key: string]: unknown;
}

export type CriteriaStatus = "pending" | "approved";

export interface Label {
  id: string;
  workspace_id: string;
  name: string;
  color: string;
}

export type IssueLinkDirection = "source" | "output";
export type IssueLinkKind = "pr" | "issue" | "branch" | "commit" | string;

export interface IssueLink {
  id: string;
  source_type: string;
  kind: IssueLinkKind;
  direction: IssueLinkDirection;
  url: string;
  external_id?: string;
  created_at: string;
}

export interface RoutineOrigin {
  run_id: string;
  routine_id: string;
  routine_name: string;
  trigger_id: string | null;
  trigger_type: "schedule" | "api" | "github" | string | null;
  source_type: string | null;
  token_prefix: string | null;
  schedule: string | null;
  timezone: string | null;
  run_at: string | null;
  action_id: string | null;
  action_type: string | null;
  event_type: string;
  triggered_at: string;
}

export interface Issue {
  id: string;
  workspace_id: string;
  number: number;
  identifier: string;
  title: string;
  description: string | null;
  status: IssueStatus;
  priority: IssuePriority;
  assignee_type: IssueAssigneeType | null;
  assignee_id: string | null;
  verifier_agent_id: string | null;
  max_verification_rounds: number | null;
  creator_type: IssueCreatorType;
  creator_id: string;
  parent_issue_id: string | null;
  acceptance_criteria: AcceptanceCriterion[];
  criteria_status: CriteriaStatus | null;
  position: number;
  due_date: string | null;
  dispatch_after: string | null;
  dispatch_provider: string | null;
  dispatch_daemon_id: string | null;
  dispatch_daemon_label: string | null;
  github_auto_fix_enabled: boolean;
  labels: Label[];
  links?: IssueLink[];
  routine_origin?: RoutineOrigin;
  reactions?: IssueReaction[];
  created_at: string;
  updated_at: string;
}
