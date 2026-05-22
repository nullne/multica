import type { Issue, IssueAssigneeType, IssuePriority } from "./issue";

export type RoutineTriggerType = "schedule" | "api" | "github";
export type RoutineActionType = "create_issue" | "comment_issue";
export type RoutineRunStatus = "processed" | "filtered" | "deduped" | "error";

export interface RoutineTrigger {
  id: string;
  routine_id: string;
  trigger_type: RoutineTriggerType;
  source_type?: string | null;
  token_prefix: string;
  installation_id?: number | null;
  schedule?: string | null;
  timezone: string;
  run_at?: string | null;
  next_run_at?: string | null;
  last_triggered_at?: string | null;
  dedup_window_seconds: number;
  max_runs?: number | null;
  successful_runs_count: number;
  config: Record<string, unknown> | null;
  enabled: boolean;
  token?: string;
  created_at: string;
  updated_at: string;
}

export interface RoutineAction {
  id: string;
  routine_id: string;
  action_type: RoutineActionType;
  config: Record<string, unknown> | null;
  enabled: boolean;
  position: number;
  created_at: string;
  updated_at: string;
}

export interface Routine {
  id: string;
  workspace_id: string;
  name: string;
  instructions?: string | null;
  priority: IssuePriority;
  assignee_type?: IssueAssigneeType | null;
  assignee_id?: string | null;
  due_date_offset_hours?: number | null;
  dispatch_provider?: string | null;
  dispatch_daemon_id?: string | null;
  dispatch_daemon_label?: string | null;
  enabled: boolean;
  created_by_id: string;
  created_by_type: string;
  subscriber_ids: string[];
  label_ids: string[];
  triggers: RoutineTrigger[];
  actions: RoutineAction[];
  created_at: string;
  updated_at: string;
}

export interface RoutineTriggerRequest {
  trigger_type: RoutineTriggerType;
  source_type?: string | null;
  schedule?: string | null;
  timezone?: string;
  run_at?: string | null;
  dedup_window_seconds?: number;
  max_runs?: number | null;
  config?: Record<string, unknown>;
  enabled?: boolean;
}

export interface RoutineActionRequest {
  action_type: RoutineActionType;
  config?: Record<string, unknown>;
  enabled?: boolean;
  position?: number;
}

export interface CreateRoutineRequest {
  name: string;
  instructions?: string | null;
  priority?: IssuePriority;
  assignee_type?: IssueAssigneeType | null;
  assignee_id?: string | null;
  due_date_offset_hours?: number | null;
  dispatch_provider?: string | null;
  dispatch_daemon_id?: string | null;
  dispatch_daemon_label?: string | null;
  enabled?: boolean;
  subscriber_ids?: string[];
  label_ids?: string[];
  triggers?: RoutineTriggerRequest[];
  actions?: RoutineActionRequest[];
}

export type UpdateRoutineRequest = CreateRoutineRequest;

export interface RoutineRun {
  id: string;
  routine_id: string;
  trigger_id: string | null;
  action_id: string | null;
  event_type: string;
  dedup_key: string;
  payload: unknown;
  status: RoutineRunStatus;
  issue_id: string | null;
  comment_id: string | null;
  error_message: string | null;
  created_at: string;
  issue?: Issue;
}
