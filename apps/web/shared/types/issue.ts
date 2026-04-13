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
  creator_type: IssueAssigneeType;
  creator_id: string;
  parent_issue_id: string | null;
  acceptance_criteria: AcceptanceCriterion[];
  criteria_status: CriteriaStatus | null;
  position: number;
  due_date: string | null;
  dispatch_provider: string | null;
  dispatch_daemon_id: string | null;
  dispatch_daemon_label: string | null;
  reactions?: IssueReaction[];
  created_at: string;
  updated_at: string;
}
