import type { IssuePriority, IssueAssigneeType } from "./issue";

export interface RecurringTemplate {
  id: string;
  workspace_id: string;
  title: string;
  description?: string;
  priority: IssuePriority;
  assignee_type?: IssueAssigneeType;
  assignee_id?: string;
  due_date_offset_hours?: number;
  dispatch_provider?: string;
  dispatch_daemon_id?: string;
  dispatch_daemon_label?: string;
  schedule: string;
  timezone: string;
  enabled: boolean;
  last_triggered_at?: string;
  next_run_at?: string;
  created_by_id: string;
  created_by_type: string;
  created_at: string;
  updated_at: string;
}

export interface CreateRecurringTemplateRequest {
  title: string;
  description?: string;
  priority?: IssuePriority;
  assignee_type?: IssueAssigneeType | null;
  assignee_id?: string | null;
  due_date_offset_hours?: number;
  schedule: string;
  timezone?: string;
  enabled?: boolean;
}

export interface UpdateRecurringTemplateRequest {
  title?: string;
  description?: string;
  priority?: IssuePriority;
  assignee_type?: IssueAssigneeType | null;
  assignee_id?: string | null;
  due_date_offset_hours?: number;
  schedule?: string;
  timezone?: string;
  enabled?: boolean;
}
