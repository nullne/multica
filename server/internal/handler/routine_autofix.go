package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	wh "github.com/nullne/multica/server/internal/webhook"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

const (
	// githubAutoFixRoutineName is the display name of the managed routine that
	// relays GitHub PR feedback back to opted-in issues.
	githubAutoFixRoutineName = "GitHub auto-fix"
	// githubAutoFixContentTemplate relays the adapter-formatted event body.
	githubAutoFixContentTemplate = "{{.body}}"
	// failedGitHubConclusions are the check_run conclusions that should
	// re-engage an agent (mirrors the legacy auto-fix behavior).
	failedGitHubConclusions = "failure,failed,error,timed_out,cancelled,action_required"
)

// executeRoutineCommentIssue runs a comment_issue routine action on the
// Handler so it can reuse the full bot-comment + agent re-engagement pipeline
// (publish, on_comment, on_mention). It iterates every issue linked to the
// event's source URL, applies the optional per-issue auto-fix gate, and posts
// the rendered comment. Returns whether at least one comment was posted.
func (h *Handler) executeRoutineCommentIssue(ctx context.Context, routine db.Routine, trigger db.RoutineTrigger, action db.RoutineAction, evt wh.Event) (bool, error) {
	var cfg CommentIssueActionConfig
	if err := json.Unmarshal(action.Config, &cfg); err != nil {
		_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, "invalid comment_issue config: "+err.Error())
		return false, fmt.Errorf("invalid comment_issue config: %w", err)
	}
	botUserID := parseUUID(cfg.BotUserID)
	if !botUserID.Valid {
		err := fmt.Errorf("comment_issue requires bot_user_id")
		_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, err.Error())
		return false, err
	}

	links, err := h.findIssueLinksForEvent(ctx, routine.WorkspaceID, evt)
	if err != nil {
		_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, err.Error())
		return false, err
	}
	if len(links) == 0 {
		_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, action.ID, evt, "filtered", pgtype.UUID{}, pgtype.UUID{}, "no matching issue link")
		return false, nil
	}

	content := renderTemplate(cfg.ContentTemplate, evt.Data)
	if content == "" {
		content = evt.Data["body"]
	}
	if content == "" {
		content = "(empty routine event)"
	}

	ran := false
	for _, link := range links {
		issue, err := h.Queries.GetIssue(ctx, link.IssueID)
		if err != nil {
			_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", link.IssueID, pgtype.UUID{}, "load issue: "+err.Error())
			continue
		}
		if cfg.OnlyIfIssueAutoFixEnabled && !issue.GithubAutoFixEnabled {
			_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, action.ID, evt, "filtered", link.IssueID, pgtype.UUID{}, "issue auto-fix disabled")
			continue
		}
		commentID, posted, err := h.createBotCommentOnIssue(ctx, issue, botUserID, content, cfg.MentionAgentID)
		if err != nil {
			_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", link.IssueID, pgtype.UUID{}, err.Error())
			continue
		}
		if posted {
			ran = true
			_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, action.ID, evt, "processed", link.IssueID, commentID, "")
		}
	}
	return ran, nil
}

// createBotCommentOnIssue posts a comment on the issue authored by the given
// bot user, broadcasts it, and runs the same agent re-engagement paths as a
// human comment (reset agent mention chain, on_comment for the assignee,
// on_mention for any @mentioned agents). Returns the comment ID and whether
// it was posted.
func (h *Handler) createBotCommentOnIssue(ctx context.Context, issue db.Issue, botUserID pgtype.UUID, content, mentionAgentID string) (pgtype.UUID, bool, error) {
	if !botUserID.Valid {
		return pgtype.UUID{}, false, fmt.Errorf("missing bot_user_id")
	}
	if content == "" {
		content = "(empty routine event)"
	}

	if mentionAgentID != "" {
		agent, agentErr := h.Queries.GetAgent(ctx, parseUUID(mentionAgentID))
		if agentErr == nil && uuidToString(agent.WorkspaceID) == uuidToString(issue.WorkspaceID) {
			content = strings.TrimRight(content, "\n") + fmt.Sprintf("\n\n[@%s](mention://agent/%s)", agent.Name, mentionAgentID)
		} else {
			slog.Warn("comment_issue: mention agent not found in workspace",
				"agent_id", mentionAgentID,
				"workspace_id", uuidToString(issue.WorkspaceID),
			)
		}
	}

	comment, err := h.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "member",
		AuthorID:    botUserID,
		Content:     content,
		Type:        "comment",
	})
	if err != nil {
		return pgtype.UUID{}, false, fmt.Errorf("create comment: %w", err)
	}

	// Broadcast the comment so live UI updates fire, then re-use the existing
	// member-comment trigger paths (on_comment for assignee, on_mention for
	// any @mentioned agents). Both functions key off authorType="member" so
	// the bot user is treated like any other human commenter.
	botID := uuidToString(botUserID)
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "member", botID, map[string]any{
		"comment":             commentForBroadcast(comment),
		"issue_title":         issue.Title,
		"issue_assignee_type": textToPtr(issue.AssigneeType),
		"issue_assignee_id":   uuidToPtr(issue.AssigneeID),
		"issue_status":        issue.Status,
	})

	// Bot-user comment counts as human intervention — reset the agent-to-agent
	// mention chain so subsequent agent dispatches are unblocked.
	if err := h.Queries.ResetIssueAgentMentionChain(ctx, issue.ID); err != nil {
		slog.Warn("routine comment: reset agent mention chain failed", "issue_id", uuidToString(issue.ID), "error", err)
	}

	onCommentEnqueued := false
	if h.shouldEnqueueOnComment(ctx, issue) && !h.commentMentionsOthersButNotAssignee(comment.Content, issue) {
		replyTo := comment.ID
		if comment.ParentID.Valid {
			replyTo = comment.ParentID
		}
		if _, err := h.TaskService.EnqueueTaskForIssue(ctx, issue, replyTo); err != nil {
			slog.Warn("routine comment: enqueue assignee task failed", "issue_id", uuidToString(issue.ID), "error", err)
		} else {
			onCommentEnqueued = true
		}
	}

	h.enqueueMentionedAgentTasks(ctx, issue, comment, "member", botID, onCommentEnqueued)

	return comment.ID, true, nil
}

// ensureGitHubBotUserExec idempotently creates (or reuses) the per-workspace
// GitHub bot user and ensures it is a member of the workspace. Runs raw SQL
// against the supplied executor so it can participate in a caller's tx.
func ensureGitHubBotUserExec(ctx context.Context, exec dbExecutor, workspaceID pgtype.UUID) (pgtype.UUID, error) {
	if exec == nil {
		return pgtype.UUID{}, fmt.Errorf("database executor unavailable")
	}
	email := fmt.Sprintf("github-bot+%s@multica.local", uuidToString(workspaceID))
	var botID pgtype.UUID
	err := exec.QueryRow(ctx, `
		WITH bot AS (
			INSERT INTO "user" (name, email, kind)
			VALUES ('GitHub', $2, 'bot')
			ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		), member_insert AS (
			INSERT INTO member (workspace_id, user_id, role)
			SELECT $1, id, 'member' FROM bot
			ON CONFLICT (workspace_id, user_id) DO NOTHING
		)
		SELECT id FROM bot
	`, workspaceID, email).Scan(&botID)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return botID, nil
}

// EnsureGitHubAutoFixRoutine idempotently provisions the managed GitHub
// auto-fix routine for a workspace: a GitHub bot user, two github triggers
// (general PR feedback + failed CI), and a comment_issue action gated by the
// per-issue auto-fix flag. Safe to call repeatedly — it no-ops when a managed
// routine already exists. The caller is responsible for transaction scope.
func EnsureGitHubAutoFixRoutine(ctx context.Context, q *db.Queries, exec dbExecutor, workspaceID pgtype.UUID, installationID pgtype.Int8) error {
	if !installationID.Valid {
		return fmt.Errorf("installation_id required to provision auto-fix routine")
	}

	botID, err := ensureGitHubBotUserExec(ctx, exec, workspaceID)
	if err != nil {
		return fmt.Errorf("ensure github bot user: %w", err)
	}

	if _, err := q.GetManagedRoutineByWorkspace(ctx, workspaceID); err == nil {
		// Already provisioned.
		return nil
	} else if err != pgx.ErrNoRows {
		return fmt.Errorf("check managed routine: %w", err)
	}

	routine, err := q.CreateManagedRoutine(ctx, db.CreateManagedRoutineParams{
		WorkspaceID:   workspaceID,
		Name:          githubAutoFixRoutineName,
		CreatedByID:   botID,
		CreatedByType: "github",
	})
	if err != nil {
		return fmt.Errorf("create managed routine: %w", err)
	}

	triggerConfigs := []routineGitHubTriggerConfig{
		{
			EventTypes: []string{
				"github.issue_comment.created",
				"github.pull_request_review_comment.created",
				"github.pull_request_review.submitted",
				"github.workflow_run.completed",
			},
			Filters: []routineTriggerFilter{
				{Field: "source_kind", Operator: "equals", Value: "pr"},
			},
		},
		{
			EventTypes: []string{"github.check_run.completed"},
			Filters: []routineTriggerFilter{
				{Field: "source_kind", Operator: "equals", Value: "pr"},
				{Field: "conclusion", Operator: "is one of", Value: failedGitHubConclusions},
			},
		},
	}
	for _, cfg := range triggerConfigs {
		configJSON, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshal trigger config: %w", err)
		}
		if _, err := q.CreateRoutineTrigger(ctx, db.CreateRoutineTriggerParams{
			RoutineID:          routine.ID,
			TriggerType:        "github",
			SourceType:         pgtype.Text{String: "github", Valid: true},
			InstallationID:     installationID,
			Timezone:           "UTC",
			DedupWindowSeconds: 600,
			Config:             configJSON,
			Enabled:            true,
		}); err != nil {
			return fmt.Errorf("create routine trigger: %w", err)
		}
	}

	actionConfig := CommentIssueActionConfig{
		ContentTemplate:           githubAutoFixContentTemplate,
		BotUserID:                 uuidToString(botID),
		OnlyIfIssueAutoFixEnabled: true,
	}
	actionJSON, err := json.Marshal(actionConfig)
	if err != nil {
		return fmt.Errorf("marshal action config: %w", err)
	}
	if _, err := q.CreateRoutineAction(ctx, db.CreateRoutineActionParams{
		RoutineID:  routine.ID,
		ActionType: "comment_issue",
		Config:     actionJSON,
		Enabled:    true,
		Position:   0,
	}); err != nil {
		return fmt.Errorf("create routine action: %w", err)
	}

	return nil
}
