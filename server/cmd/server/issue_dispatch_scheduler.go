package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/nullne/multica/server/internal/service"
	"github.com/nullne/multica/server/internal/util"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

const (
	issueDispatchInterval  = 60 * time.Second
	issueDispatchBatchSize = int32(50)
)

// runIssueDispatchScheduler periodically enqueues agent tasks for issues whose
// dispatch_after time has arrived. Assigning an agent to an issue with a
// future dispatch_after defers the enqueue to this scheduler instead of doing
// it inline in the handler.
func runIssueDispatchScheduler(ctx context.Context, queries *db.Queries, taskSvc *service.TaskService) {
	ticker := time.NewTicker(issueDispatchInterval)
	defer ticker.Stop()

	fireDueIssueDispatches(ctx, queries, taskSvc)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fireDueIssueDispatches(ctx, queries, taskSvc)
		}
	}
}

func fireDueIssueDispatches(ctx context.Context, queries *db.Queries, taskSvc *service.TaskService) {
	due, err := queries.ListDueDispatchIssues(ctx, issueDispatchBatchSize)
	if err != nil {
		slog.Warn("issue dispatch scheduler: list due issues failed", "error", err)
		return
	}
	for _, issue := range due {
		fireIssueDispatch(ctx, queries, taskSvc, issue)
	}
}

func fireIssueDispatch(ctx context.Context, queries *db.Queries, taskSvc *service.TaskService, issue db.Issue) {
	// CAS claim: marks the schedule as consumed so concurrent scheduler
	// instances (or overlapping ticks) never enqueue twice.
	claimed, err := queries.ClaimIssueDispatch(ctx, issue.ID)
	if err != nil {
		slog.Debug("issue dispatch scheduler: dispatch already claimed or rescheduled", "issue_id", util.UUIDToString(issue.ID))
		return
	}

	// Same gate as the handler's assign-time check: the assigned agent must
	// exist, be active, and have the on_assign trigger enabled. When the gate
	// fails the claim stays consumed — the schedule was honored, the agent
	// just opted out of assignment triggers.
	agent, err := queries.GetAgent(ctx, claimed.AssigneeID)
	if err != nil || len(agent.Providers) == 0 || agent.ArchivedAt.Valid {
		slog.Info("issue dispatch scheduler: assignee agent not dispatchable", "issue_id", util.UUIDToString(claimed.ID), "error", err)
		return
	}
	if !service.AgentHasTriggerEnabled(agent.Triggers, "on_assign") {
		slog.Info("issue dispatch scheduler: on_assign trigger disabled", "issue_id", util.UUIDToString(claimed.ID), "agent_id", util.UUIDToString(agent.ID))
		return
	}

	if _, err := taskSvc.EnqueueTaskForIssue(ctx, claimed); err != nil {
		// Transient failures (typically no online runtime) release the claim
		// so the next tick retries.
		if resetErr := queries.ResetIssueDispatchFired(ctx, claimed.ID); resetErr != nil {
			slog.Warn("issue dispatch scheduler: release claim failed", "issue_id", util.UUIDToString(claimed.ID), "error", resetErr)
		}
		slog.Warn("issue dispatch scheduler: enqueue failed, will retry", "issue_id", util.UUIDToString(claimed.ID), "error", err)
		return
	}

	slog.Info("issue dispatch scheduler: task enqueued", "issue_id", util.UUIDToString(claimed.ID), "agent_id", util.UUIDToString(agent.ID))
}
