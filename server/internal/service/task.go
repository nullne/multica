package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/events"
	"github.com/nullne/multica/server/internal/mention"
	"github.com/nullne/multica/server/internal/realtime"
	"github.com/nullne/multica/server/internal/util"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
	"github.com/nullne/multica/server/pkg/redact"
)

type TaskService struct {
	Queries *db.Queries
	Hub     *realtime.Hub
	Bus     *events.Bus
}

func NewTaskService(q *db.Queries, hub *realtime.Hub, bus *events.Bus) *TaskService {
	return &TaskService{Queries: q, Hub: hub, Bus: bus}
}

const (
	verificationFlowName      = "verification_loop"
	taskRoleCriteria          = "criteria"
	taskRoleExecutor          = "executor"
	taskRoleValidator         = "validator"
	taskRoleRework            = "rework"
	defaultVerificationRounds = 5
)

var (
	criteriaBlockPattern     = regexp.MustCompile(`(?s)<!--\s*multica:criteria\s*(\{.*?\})\s*-->`)
	verificationBlockPattern = regexp.MustCompile(`(?s)<!--\s*multica:verification\s*(\{.*?\})\s*-->`)
)

type verificationTaskContext struct {
	Flow                string                    `json:"flow,omitempty"`
	Role                string                    `json:"role,omitempty"`
	Round               int                       `json:"round,omitempty"`
	ExecutorAgentID     string                    `json:"executor_agent_id,omitempty"`
	VerifierAgentID     string                    `json:"verifier_agent_id,omitempty"`
	AcceptanceCriteria  []map[string]any          `json:"acceptance_criteria,omitempty"`
	VerificationSummary string                    `json:"verification_summary,omitempty"`
	FailedChecks        []verificationFailedCheck `json:"failed_checks,omitempty"`
}

type criteriaPayload struct {
	Criteria []map[string]any `json:"criteria"`
}

type verificationPayload struct {
	Decision     string                    `json:"decision"`
	Summary      string                    `json:"summary"`
	FailedChecks []verificationFailedCheck `json:"failed_checks"`
}

type verificationFailedCheck struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// EnqueueTaskForIssue creates a queued task for an agent-assigned issue.
// No context snapshot is stored — the agent fetches all data it needs at
// runtime via the multica CLI.
func (s *TaskService) EnqueueTaskForIssue(ctx context.Context, issue db.Issue, triggerCommentID ...pgtype.UUID) (db.AgentTaskQueue, error) {
	if !issue.AssigneeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "issue has no assignee")
		return db.AgentTaskQueue{}, fmt.Errorf("issue has no assignee")
	}

	var commentID pgtype.UUID
	if len(triggerCommentID) > 0 {
		commentID = triggerCommentID[0]
	}

	// Verification loop applies only to assignment-triggered tasks.
	if !commentID.Valid &&
		issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" &&
		issue.VerifierAgentID.Valid &&
		util.UUIDToString(issue.VerifierAgentID) != util.UUIDToString(issue.AssigneeID) {
		criteria := decodeAcceptanceCriteria(issue.AcceptanceCriteria)
		if len(criteria) == 0 || !issue.CriteriaStatus.Valid {
			// No criteria yet — enqueue criteria generation to verifier.
			contextData := buildVerificationTaskContext(verificationTaskContext{
				Flow:            verificationFlowName,
				Role:            taskRoleCriteria,
				Round:           1,
				ExecutorAgentID: util.UUIDToString(issue.AssigneeID),
				VerifierAgentID: util.UUIDToString(issue.VerifierAgentID),
			})
			return s.enqueueTaskToAgent(ctx, issue, issue.VerifierAgentID, commentID, contextData, "criteria task enqueued")
		}
		if issue.CriteriaStatus.String == "pending" {
			// Criteria exist but not yet approved — wait for human approval.
			slog.Info("criteria pending approval, not enqueueing executor", "issue_id", util.UUIDToString(issue.ID))
			return db.AgentTaskQueue{}, nil
		}

		// Criteria approved — enqueue executor.
		contextData := buildVerificationTaskContext(verificationTaskContext{
			Flow:               verificationFlowName,
			Role:               taskRoleExecutor,
			Round:              1,
			ExecutorAgentID:    util.UUIDToString(issue.AssigneeID),
			VerifierAgentID:    util.UUIDToString(issue.VerifierAgentID),
			AcceptanceCriteria: criteria,
		})
		return s.enqueueTaskToAgent(ctx, issue, issue.AssigneeID, commentID, contextData, "executor task enqueued")
	}

	return s.enqueueTaskToAgent(ctx, issue, issue.AssigneeID, commentID, nil, "task enqueued")
}

// EnqueueExecutorForApprovedCriteria enqueues the executor task after criteria
// have been approved by a human. Called from the criteria approve handler.
func (s *TaskService) EnqueueExecutorForApprovedCriteria(ctx context.Context, issue db.Issue) (db.AgentTaskQueue, error) {
	if !issue.AssigneeID.Valid || !issue.VerifierAgentID.Valid {
		return db.AgentTaskQueue{}, fmt.Errorf("issue must have both assignee and verifier")
	}
	criteria := decodeAcceptanceCriteria(issue.AcceptanceCriteria)
	contextData := buildVerificationTaskContext(verificationTaskContext{
		Flow:               verificationFlowName,
		Role:               taskRoleExecutor,
		Round:              1,
		ExecutorAgentID:    util.UUIDToString(issue.AssigneeID),
		VerifierAgentID:    util.UUIDToString(issue.VerifierAgentID),
		AcceptanceCriteria: criteria,
	})
	return s.enqueueTaskToAgent(ctx, issue, issue.AssigneeID, pgtype.UUID{}, contextData, "executor task enqueued")
}

// EnqueueTaskForMention creates a queued task for a mentioned agent on an issue.
// Unlike EnqueueTaskForIssue, this takes an explicit agent ID rather than
// deriving it from the issue assignee.
func (s *TaskService) EnqueueTaskForMention(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.enqueueTaskToAgent(ctx, issue, agentID, triggerCommentID, nil, "mention task enqueued")
}

func (s *TaskService) enqueueTaskToAgent(ctx context.Context, issue db.Issue, agentID, triggerCommentID pgtype.UUID, contextData []byte, logMsg string) (db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Error("mention task enqueue failed: agent not found", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("mention task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if len(agent.Providers) == 0 {
		slog.Error("task enqueue failed: agent has no providers", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no providers configured")
	}

	// Use issue-level dispatch hints to constrain runtime selection.
	params := db.FindAvailableRuntimeConstrainedParams{
		WorkspaceID: issue.WorkspaceID,
		Providers:   agent.Providers,
	}
	if issue.DispatchProvider.Valid {
		params.Provider = issue.DispatchProvider
	}
	if issue.DispatchDaemonID.Valid {
		params.DaemonID = issue.DispatchDaemonID
	} else if agent.DefaultDaemonID.Valid {
		params.DaemonID = agent.DefaultDaemonID
	}
	if issue.DispatchDaemonLabel.Valid {
		params.DaemonLabel = issue.DispatchDaemonLabel
	}

	runtime, err := s.Queries.FindAvailableRuntimeConstrained(ctx, params)
	if err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("no online runtime for providers %v (dispatch hints: provider=%q valid=%v, daemon_id=%s valid=%v, daemon_label=%q valid=%v)",
			agent.Providers,
			issue.DispatchProvider.String, issue.DispatchProvider.Valid,
			util.UUIDToString(issue.DispatchDaemonID), issue.DispatchDaemonID.Valid,
			issue.DispatchDaemonLabel.String, issue.DispatchDaemonLabel.Valid,
		)
	}

	runtimeID := runtime.ID

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:          agentID,
		RuntimeID:        runtimeID,
		IssueID:          issue.ID,
		Priority:         priorityToInt(issue.Priority),
		TriggerCommentID: triggerCommentID,
		Context:          contextData,
	})
	if err != nil {
		slog.Error("mention task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info(logMsg, "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
	return task, nil
}

// CancelTasksForIssue cancels all active tasks for an issue.
func (s *TaskService) CancelTasksForIssue(ctx context.Context, issueID pgtype.UUID) error {
	return s.Queries.CancelAgentTasksByIssue(ctx, issueID)
}

// CancelTask cancels a single task by ID. It broadcasts a task:cancelled event
// so frontends can update immediately.
func (s *TaskService) CancelTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.CancelAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("cancel task: %w", err)
	}

	slog.Info("task cancelled", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast cancellation as a task:failed event so frontends clear the live card
	s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task)

	return &task, nil
}

// ClaimTask atomically claims the next queued task for an agent,
// respecting max_concurrent_tasks.
func (s *TaskService) ClaimTask(ctx context.Context, agentID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.ClaimAgentTask(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("task claim: no tasks available", "agent_id", util.UUIDToString(agentID))
			return nil, nil // No tasks available
		}
		return nil, fmt.Errorf("claim task: %w", err)
	}

	slog.Info("task claimed", "task_id", util.UUIDToString(task.ID), "agent_id", util.UUIDToString(agentID))

	// Update agent status to working
	s.updateAgentStatus(ctx, agentID, "working")

	// Broadcast task:dispatch
	s.broadcastTaskDispatch(ctx, task)

	return &task, nil
}

// ClaimTaskForRuntime claims the next runnable task for a runtime while
// still respecting each agent's max_concurrent_tasks limit.
func (s *TaskService) ClaimTaskForRuntime(ctx context.Context, runtimeID pgtype.UUID) (*db.AgentTaskQueue, error) {
	tasks, err := s.Queries.ListPendingTasksByRuntime(ctx, runtimeID)
	if err != nil {
		return nil, fmt.Errorf("list pending tasks: %w", err)
	}

	triedAgents := map[string]struct{}{}
	for _, candidate := range tasks {
		agentKey := util.UUIDToString(candidate.AgentID)
		if _, seen := triedAgents[agentKey]; seen {
			continue
		}
		triedAgents[agentKey] = struct{}{}

		task, err := s.ClaimTask(ctx, candidate.AgentID)
		if err != nil {
			return nil, err
		}
		if task != nil && task.RuntimeID == runtimeID {
			return task, nil
		}
	}

	return nil, nil
}

// StartTask transitions a dispatched task to running.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) StartTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.StartAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("start task: %w", err)
	}

	slog.Info("task started", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	return &task, nil
}

// CompleteTask marks a task as completed.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) CompleteTask(ctx context.Context, taskID pgtype.UUID, result []byte, sessionID, workDir string) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.CompleteAgentTask(ctx, db.CompleteAgentTaskParams{
		ID:        taskID,
		Result:    result,
		SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
		WorkDir:   pgtype.Text{String: workDir, Valid: workDir != ""},
	})
	if err != nil {
		// Log the current task state to help debug why the update matched no rows.
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, taskID); lookupErr == nil {
			slog.Warn("complete task failed: task not in running state",
				"task_id", util.UUIDToString(taskID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"agent_id", util.UUIDToString(existing.AgentID),
			)
		} else {
			slog.Warn("complete task failed: task not found",
				"task_id", util.UUIDToString(taskID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("complete task: %w", err)
	}

	slog.Info("task completed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))

	var payload protocol.TaskCompletedPayload
	if err := json.Unmarshal(result, &payload); err != nil {
		slog.Debug("task completion payload parse failed", "task_id", util.UUIDToString(task.ID), "error", err)
	}
	if payload.BranchName != "" || payload.PRURL != "" {
		// Persist agent-produced links (PR and branch) into issue_link as
		// outgoing references. Branch URLs aren't real URLs, so we synthesize
		// a stable identifier "branch:<repo>?<branch>" to satisfy the unique
		// (workspace_id, url) constraint while still being identifiable.
		issue, issueErr := s.Queries.GetIssue(ctx, task.IssueID)
		if issueErr != nil {
			slog.Warn("load issue for dev-link write failed",
				"issue_id", util.UUIDToString(task.IssueID),
				"task_id", util.UUIDToString(task.ID),
				"error", issueErr,
			)
		} else {
			if payload.PRURL != "" {
				if _, err := s.Queries.CreateIssueLink(ctx, db.CreateIssueLinkParams{
					IssueID:     issue.ID,
					WorkspaceID: issue.WorkspaceID,
					SourceType:  "github",
					Kind:        "pr",
					Direction:   "output",
					Url:         payload.PRURL,
					ExternalID:  "",
				}); err != nil {
					slog.Warn("create outgoing pr link failed",
						"issue_id", util.UUIDToString(issue.ID),
						"task_id", util.UUIDToString(task.ID),
						"error", err,
					)
				}
			}
			if payload.BranchName != "" {
				if _, err := s.Queries.CreateIssueLink(ctx, db.CreateIssueLinkParams{
					IssueID:     issue.ID,
					WorkspaceID: issue.WorkspaceID,
					SourceType:  "github",
					Kind:        "branch",
					Direction:   "output",
					Url:         "branch:" + payload.BranchName,
					ExternalID:  payload.BranchName,
				}); err != nil {
					slog.Warn("create outgoing branch link failed",
						"issue_id", util.UUIDToString(issue.ID),
						"task_id", util.UUIDToString(task.ID),
						"error", err,
					)
				}
			}
			s.broadcastIssueUpdated(issue)
		}
	}

	output := redact.Text(payload.Output)

	taskCtx := parseVerificationTaskContext(task.Context)
	handledByVerificationFlow := false
	if taskCtx.Flow == verificationFlowName && taskCtx.Role != "" {
		handledByVerificationFlow = true
		if err := s.handleVerificationCompletion(ctx, task, taskCtx, output); err != nil {
			slog.Warn("verification completion handling failed", "task_id", util.UUIDToString(task.ID), "role", taskCtx.Role, "error", err)
			s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text("Verification flow handling failed: "+err.Error()), "system", task.TriggerCommentID)
		}
	}

	// Post agent output as a comment for non-verification-flow assignment tasks.
	// Comment-triggered tasks: the agent replies via CLI with --parent, so
	// posting here would create a duplicate.
	if !handledByVerificationFlow && !task.TriggerCommentID.Valid && output != "" {
		s.createAgentComment(ctx, task.IssueID, task.AgentID, output, "comment", task.TriggerCommentID)
	}

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskCompleted, task)

	return &task, nil
}

func (s *TaskService) handleVerificationCompletion(ctx context.Context, task db.AgentTaskQueue, taskCtx verificationTaskContext, output string) error {
	switch taskCtx.Role {
	case taskRoleCriteria:
		criteria, err := extractCriteriaPayload(output)
		if err != nil || len(criteria) == 0 {
			s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text("验收标准解析失败：请按 `<!--multica:criteria ... -->` 结构化格式输出。"), "system", task.TriggerCommentID)
			return nil
		}

		criteriaJSON, err := json.Marshal(criteria)
		if err != nil {
			return fmt.Errorf("marshal criteria: %w", err)
		}
		issue, err := s.Queries.UpdateIssueAcceptanceCriteria(ctx, db.UpdateIssueAcceptanceCriteriaParams{
			ID:                 task.IssueID,
			AcceptanceCriteria: criteriaJSON,
			CriteriaStatus:     pgtype.Text{String: "pending", Valid: true},
		})
		if err != nil {
			return fmt.Errorf("save acceptance criteria: %w", err)
		}

		comment := stripMachineBlocks(output)
		if comment == "" {
			comment = "验收标准已起草，等待确认。"
		}
		s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(comment), "comment", task.TriggerCommentID)
		s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text("验收标准已生成，请在 issue 详情页确认后继续。"), "system", task.TriggerCommentID)

		s.broadcastIssueUpdated(issue)
		return nil

	case taskRoleExecutor, taskRoleRework:
		issue, err := s.Queries.GetIssue(ctx, task.IssueID)
		if err != nil {
			return fmt.Errorf("load issue: %w", err)
		}
		if output != "" {
			s.createAgentComment(ctx, task.IssueID, task.AgentID, output, "comment", task.TriggerCommentID)
		}

		if !issue.VerifierAgentID.Valid || !issue.AssigneeID.Valid ||
			util.UUIDToString(issue.VerifierAgentID) == util.UUIDToString(issue.AssigneeID) {
			return nil
		}
		nextCtx := buildVerificationTaskContext(verificationTaskContext{
			Flow:               verificationFlowName,
			Role:               taskRoleValidator,
			Round:              maxRound(taskCtx.Round),
			ExecutorAgentID:    util.UUIDToString(issue.AssigneeID),
			VerifierAgentID:    util.UUIDToString(issue.VerifierAgentID),
			AcceptanceCriteria: decodeAcceptanceCriteria(issue.AcceptanceCriteria),
		})
		_, err = s.enqueueTaskToAgent(ctx, issue, issue.VerifierAgentID, pgtype.UUID{}, nextCtx, "validator task enqueued")
		return err

	case taskRoleValidator:
		issue, err := s.Queries.GetIssue(ctx, task.IssueID)
		if err != nil {
			return fmt.Errorf("load issue: %w", err)
		}

		result, err := extractVerificationPayload(output)
		if err != nil {
			s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text("验收结果解析失败：请按 `<!--multica:verification ... -->` 结构化格式输出。"), "system", task.TriggerCommentID)
			return nil
		}

		decision := strings.ToLower(strings.TrimSpace(result.Decision))
		humanOutput := stripMachineBlocks(output)
		if humanOutput == "" {
			humanOutput = output
		}

		switch decision {
		case "pass":
			if humanOutput == "" {
				humanOutput = "验收通过。"
			}
			s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(humanOutput), "comment", task.TriggerCommentID)
			if issue.Status == "in_review" {
				if updated, err := s.Queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: issue.ID, Status: "done"}); err == nil {
					s.broadcastIssueUpdated(updated)
				}
			}
			return nil

		case "fail":
			if !issue.AssigneeID.Valid {
				s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text("验收未通过，但当前 issue 没有执行 agent，无法自动回流。"), "system", task.TriggerCommentID)
				return nil
			}

			assigneeMention := fmt.Sprintf("[@执行Agent](mention://agent/%s)", util.UUIDToString(issue.AssigneeID))
			feedback := strings.TrimSpace(humanOutput)
			if feedback == "" {
				feedback = "验收未通过。"
			}
			if !strings.Contains(feedback, "mention://agent/"+util.UUIDToString(issue.AssigneeID)) {
				feedback += "\n\n请修复后再次提交：" + assigneeMention
			}
			s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(feedback), "comment", task.TriggerCommentID)

			round := maxRound(taskCtx.Round)
			maxRounds := defaultVerificationRounds
			if issue.MaxVerificationRounds.Valid && issue.MaxVerificationRounds.Int32 > 0 {
				maxRounds = int(issue.MaxVerificationRounds.Int32)
			}
			if round >= maxRounds {
				s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(fmt.Sprintf("自动修复达到最大轮次（%d），已停止自动回流。", maxRounds)), "system", task.TriggerCommentID)
				return nil
			}

			nextCtx := buildVerificationTaskContext(verificationTaskContext{
				Flow:                verificationFlowName,
				Role:                taskRoleRework,
				Round:               round + 1,
				ExecutorAgentID:     util.UUIDToString(issue.AssigneeID),
				VerifierAgentID:     util.UUIDToString(task.AgentID),
				AcceptanceCriteria:  decodeAcceptanceCriteria(issue.AcceptanceCriteria),
				VerificationSummary: result.Summary,
				FailedChecks:        result.FailedChecks,
			})
			_, err = s.enqueueTaskToAgent(ctx, issue, issue.AssigneeID, pgtype.UUID{}, nextCtx, "rework task enqueued")
			return err

		default:
			s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text("验收结果中 decision 必须为 pass 或 fail。"), "system", task.TriggerCommentID)
			return nil
		}
	}

	return nil
}

// FailTask marks a task as failed.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) FailTask(ctx context.Context, taskID pgtype.UUID, errMsg string) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.FailAgentTask(ctx, db.FailAgentTaskParams{
		ID:    taskID,
		Error: pgtype.Text{String: errMsg, Valid: true},
	})
	if err != nil {
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, taskID); lookupErr == nil {
			slog.Warn("fail task failed: task not in dispatched/running state",
				"task_id", util.UUIDToString(taskID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"agent_id", util.UUIDToString(existing.AgentID),
			)
		} else {
			slog.Warn("fail task failed: task not found",
				"task_id", util.UUIDToString(taskID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("fail task: %w", err)
	}

	slog.Warn("task failed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID), "error", errMsg)

	if errMsg != "" {
		s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(errMsg), "system", task.TriggerCommentID)
	}
	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskFailed, task)

	return &task, nil
}

// ReportProgress broadcasts a progress update via the event bus.
func (s *TaskService) ReportProgress(ctx context.Context, taskID string, workspaceID string, summary string, step, total int) {
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskProgress,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: protocol.TaskProgressPayload{
			TaskID:  taskID,
			Summary: summary,
			Step:    step,
			Total:   total,
		},
	})
}

// ReconcileAgentStatus checks running task count and sets agent status accordingly.
func (s *TaskService) ReconcileAgentStatus(ctx context.Context, agentID pgtype.UUID) {
	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return
	}
	newStatus := "idle"
	if running > 0 {
		newStatus = "working"
	}
	slog.Debug("agent status reconciled", "agent_id", util.UUIDToString(agentID), "status", newStatus, "running_tasks", running)
	s.updateAgentStatus(ctx, agentID, newStatus)
}

func (s *TaskService) updateAgentStatus(ctx context.Context, agentID pgtype.UUID, status string) {
	agent, err := s.Queries.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{
		ID:     agentID,
		Status: status,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventAgentStatus,
		WorkspaceID: util.UUIDToString(agent.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"agent": agentToMap(agent)},
	})
}

// LoadAgentSkills loads an agent's skills with their files for task execution.
func (s *TaskService) LoadAgentSkills(ctx context.Context, agentID pgtype.UUID) []AgentSkillData {
	skills, err := s.Queries.ListAgentSkills(ctx, agentID)
	if err != nil || len(skills) == 0 {
		return nil
	}

	result := make([]AgentSkillData, 0, len(skills))
	for _, sk := range skills {
		data := AgentSkillData{Name: sk.Name, Content: sk.Content}
		files, _ := s.Queries.ListSkillFiles(ctx, sk.ID)
		for _, f := range files {
			data.Files = append(data.Files, AgentSkillFileData{Path: f.Path, Content: f.Content})
		}
		result = append(result, data)
	}
	return result
}

// AgentSkillData represents a skill for task execution responses.
type AgentSkillData struct {
	Name    string               `json:"name"`
	Content string               `json:"content"`
	Files   []AgentSkillFileData `json:"files,omitempty"`
}

// AgentSkillFileData represents a supporting file within a skill.
type AgentSkillFileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func parseVerificationTaskContext(raw []byte) verificationTaskContext {
	if len(raw) == 0 {
		return verificationTaskContext{}
	}
	var ctx verificationTaskContext
	if err := json.Unmarshal(raw, &ctx); err != nil {
		return verificationTaskContext{}
	}
	return ctx
}

func buildVerificationTaskContext(ctx verificationTaskContext) []byte {
	if ctx.Role == "" {
		return nil
	}
	b, err := json.Marshal(ctx)
	if err != nil {
		return nil
	}
	return b
}

func decodeAcceptanceCriteria(raw []byte) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var criteria []map[string]any
	if err := json.Unmarshal(raw, &criteria); err != nil {
		return nil
	}
	return criteria
}

func extractCriteriaPayload(output string) ([]map[string]any, error) {
	m := criteriaBlockPattern.FindStringSubmatch(output)
	if len(m) < 2 {
		return nil, fmt.Errorf("criteria block not found")
	}
	var payload criteriaPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &payload); err != nil {
		return nil, fmt.Errorf("parse criteria payload: %w", err)
	}
	return payload.Criteria, nil
}

func extractVerificationPayload(output string) (verificationPayload, error) {
	m := verificationBlockPattern.FindStringSubmatch(output)
	if len(m) < 2 {
		return verificationPayload{}, fmt.Errorf("verification block not found")
	}
	var payload verificationPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &payload); err != nil {
		return verificationPayload{}, fmt.Errorf("parse verification payload: %w", err)
	}
	return payload, nil
}

func stripMachineBlocks(output string) string {
	if output == "" {
		return ""
	}
	stripped := criteriaBlockPattern.ReplaceAllString(output, "")
	stripped = verificationBlockPattern.ReplaceAllString(stripped, "")
	return strings.TrimSpace(stripped)
}

func maxRound(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func priorityToInt(p string) int32 {
	switch p {
	case "urgent":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func (s *TaskService) broadcastTaskDispatch(ctx context.Context, task db.AgentTaskQueue) {
	var payload map[string]any
	if task.Context != nil {
		json.Unmarshal(task.Context, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["task_id"] = util.UUIDToString(task.ID)
	payload["runtime_id"] = util.UUIDToString(task.RuntimeID)

	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return // Issue deleted; skip broadcast to avoid global leak
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskDispatch,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})
}

func (s *TaskService) broadcastTaskEvent(ctx context.Context, eventType string, task db.AgentTaskQueue) {
	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return // Issue deleted; skip broadcast to avoid global leak
	}
	s.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"task_id":  util.UUIDToString(task.ID),
			"agent_id": util.UUIDToString(task.AgentID),
			"issue_id": util.UUIDToString(task.IssueID),
			"status":   task.Status,
		},
	})
}

func (s *TaskService) broadcastIssueUpdated(issue db.Issue) {
	prefix := s.getIssuePrefix(issue.WorkspaceID)
	s.Bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"issue": issueToMap(issue, prefix)},
	})
}

func (s *TaskService) getIssuePrefix(workspaceID pgtype.UUID) string {
	ws, err := s.Queries.GetWorkspace(context.Background(), workspaceID)
	if err != nil {
		return ""
	}
	return ws.IssuePrefix
}

func (s *TaskService) createAgentComment(ctx context.Context, issueID, agentID pgtype.UUID, content, commentType string, parentID pgtype.UUID) {
	if content == "" {
		return
	}
	// Look up issue to get workspace ID for mention expansion and broadcasting.
	issue, err := s.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return
	}
	// Expand bare issue identifiers (e.g. MUL-117) into mention links.
	content = mention.ExpandIssueIdentifiers(ctx, s.Queries, issue.WorkspaceID, content)
	comment, err := s.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:    issueID,
		AuthorType: "agent",
		AuthorID:   agentID,
		Content:    content,
		Type:       commentType,
		ParentID:   parentID,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(agentID),
		Payload: map[string]any{
			"comment": map[string]any{
				"id":          util.UUIDToString(comment.ID),
				"issue_id":    util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType,
				"author_id":   util.UUIDToString(comment.AuthorID),
				"content":     comment.Content,
				"type":        comment.Type,
				"parent_id":   util.UUIDToPtr(comment.ParentID),
				"created_at":  comment.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
			},
			"issue_title":  issue.Title,
			"issue_status": issue.Status,
		},
	})
}

func issueToMap(issue db.Issue, issuePrefix string) map[string]any {
	acceptanceCriteria := []any{}
	if issue.AcceptanceCriteria != nil {
		_ = json.Unmarshal(issue.AcceptanceCriteria, &acceptanceCriteria)
	}
	return map[string]any{
		"id":                  util.UUIDToString(issue.ID),
		"workspace_id":        util.UUIDToString(issue.WorkspaceID),
		"number":              issue.Number,
		"identifier":          issuePrefix + "-" + strconv.Itoa(int(issue.Number)),
		"title":               issue.Title,
		"description":         util.TextToPtr(issue.Description),
		"status":              issue.Status,
		"priority":            issue.Priority,
		"assignee_type":       util.TextToPtr(issue.AssigneeType),
		"assignee_id":         util.UUIDToPtr(issue.AssigneeID),
		"verifier_agent_id":   util.UUIDToPtr(issue.VerifierAgentID),
		"creator_type":        issue.CreatorType,
		"creator_id":          util.UUIDToString(issue.CreatorID),
		"parent_issue_id":     util.UUIDToPtr(issue.ParentIssueID),
		"acceptance_criteria": acceptanceCriteria,
		"position":            issue.Position,
		"due_date":            util.TimestampToPtr(issue.DueDate),
		"created_at":          util.TimestampToString(issue.CreatedAt),
		"updated_at":          util.TimestampToString(issue.UpdatedAt),
	}
}

// agentToMap builds a simple map for broadcasting agent status updates.
func agentToMap(a db.Agent) map[string]any {
	var tools any
	if a.Tools != nil {
		json.Unmarshal(a.Tools, &tools)
	}
	var triggers any
	if a.Triggers != nil {
		json.Unmarshal(a.Triggers, &triggers)
	}
	return map[string]any{
		"id":           util.UUIDToString(a.ID),
		"workspace_id": util.UUIDToString(a.WorkspaceID),
		"providers":    a.Providers,
		"name":         a.Name,
		"description":  a.Description,
		"avatar_url":   util.TextToPtr(a.AvatarUrl),
		"visibility":   a.Visibility,
		"status":       a.Status,
		"owner_id":     util.UUIDToPtr(a.OwnerID),
		"skills":       []any{},
		"tools":        tools,
		"triggers":     triggers,
		"created_at":   util.TimestampToString(a.CreatedAt),
		"updated_at":   util.TimestampToString(a.UpdatedAt),
		"archived_at":  util.TimestampToPtr(a.ArchivedAt),
		"archived_by":  util.UUIDToPtr(a.ArchivedBy),
	}
}
