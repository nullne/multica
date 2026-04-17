package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

// --- GitHub Event Receiver (public, signature-verified) ---

// ReceiveGitHubEvent handles incoming GitHub App webhook events. It is
// a public endpoint authenticated via HMAC signature verification
// (X-Hub-Signature-256) rather than JWT.
func (h *Handler) ReceiveGitHubEvent(w http.ResponseWriter, r *http.Request) {
	if h.GitHubApp == nil {
		writeError(w, http.StatusNotImplemented, "GitHub App not configured")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	if !h.GitHubApp.VerifySignature(body, r.Header.Get("X-Hub-Signature-256")) {
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		writeError(w, http.StatusBadRequest, "missing X-GitHub-Event header")
		return
	}

	// Ping event is a connectivity check from GitHub — just respond OK.
	if eventType == "ping" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pong"})
		return
	}

	var envelope struct {
		Action       string `json:"action"`
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	if envelope.Installation.ID == 0 {
		writeError(w, http.StatusBadRequest, "missing installation.id")
		return
	}

	if !isRelevantAction(eventType, envelope.Action) {
		w.WriteHeader(http.StatusOK)
		return
	}

	ws, err := h.Queries.GetWorkspaceByInstallationID(r.Context(), pgtype.Int8{Int64: envelope.Installation.ID, Valid: true})
	if err != nil {
		slog.Debug("github event: no workspace for installation", "installation_id", envelope.Installation.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	rule, err := h.Queries.GetGitHubEventRule(r.Context(), db.GetGitHubEventRuleParams{
		WorkspaceID: ws.ID,
		EventType:   eventType,
	})
	if err != nil || !rule.Enabled {
		w.WriteHeader(http.StatusOK)
		return
	}

	data := parseGitHubEventData(eventType, body)

	titleTmpl := rule.TitleTemplate
	if titleTmpl == "" {
		titleTmpl = defaultTitleTemplate(eventType)
	}
	title := renderTemplate(titleTmpl, data)
	if title == "" {
		title = data["title"]
	}
	if title == "" {
		title = fmt.Sprintf("GitHub %s event", eventType)
	}

	descTmpl := rule.DescriptionTemplate
	if descTmpl == "" {
		descTmpl = defaultDescriptionTemplate(eventType)
	}
	description := renderTemplate(descTmpl, data)
	if description == "" {
		description = data["body"]
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		slog.Error("github event: failed to begin tx", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)
	issueNumber, err := qtx.IncrementIssueCounter(r.Context(), ws.ID)
	if err != nil {
		slog.Error("github event: failed to increment issue counter", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	issue, err := qtx.CreateIssue(r.Context(), db.CreateIssueParams{
		WorkspaceID:         ws.ID,
		Title:               title,
		Description:         strToText(description),
		Status:              "backlog",
		Priority:            "medium",
		AssigneeType:        pgtype.Text{String: "agent", Valid: true},
		AssigneeID:          rule.AgentID,
		CreatorType:         "github",
		CreatorID:           rule.ID,
		Number:              issueNumber,
		DispatchProvider:    rule.DispatchProvider,
		DispatchDaemonID:    rule.DispatchDaemonID,
		DispatchDaemonLabel: rule.DispatchDaemonLabel,
	})
	if err != nil {
		slog.Error("github event: failed to create issue", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create issue")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		slog.Error("github event: failed to commit", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	workspaceID := uuidToString(ws.ID)
	prefix := h.getIssuePrefix(r.Context(), ws.ID)
	resp := issueToResponse(issue, prefix)
	h.publish(protocol.EventIssueCreated, workspaceID, "system", "", map[string]any{"issue": resp})

	if h.shouldEnqueueAgentTask(r.Context(), issue) {
		if _, err := h.TaskService.EnqueueTaskForIssue(r.Context(), issue); err != nil {
			slog.Warn("github event: enqueue agent task failed", "issue_id", uuidToString(issue.ID), "error", err)
		}
	}

	slog.Info("github event: issue created",
		"event_type", eventType,
		"action", envelope.Action,
		"issue_id", uuidToString(issue.ID),
		"workspace_id", workspaceID,
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"issue_id": uuidToString(issue.ID),
	})
}

// isRelevantAction filters GitHub event actions to only process meaningful ones.
func isRelevantAction(eventType, action string) bool {
	switch eventType {
	case "push":
		return true
	case "pull_request":
		switch action {
		case "opened", "synchronize", "reopened", "closed":
			return true
		}
		return false
	case "issues":
		switch action {
		case "opened", "reopened":
			return true
		}
		return false
	case "issue_comment":
		return action == "created"
	default:
		return false
	}
}

// --- Event data parsers ---

func parseGitHubEventData(eventType string, body []byte) map[string]string {
	switch eventType {
	case "push":
		return parsePushEvent(body)
	case "pull_request":
		return parsePullRequestEvent(body)
	case "issues":
		return parseIssuesEvent(body)
	case "issue_comment":
		return parseIssueCommentEvent(body)
	default:
		return map[string]string{}
	}
}

func parsePushEvent(body []byte) map[string]string {
	var ev struct {
		Ref     string `json:"ref"`
		Compare string `json:"compare"`
		Pusher  struct {
			Name string `json:"name"`
		} `json:"pusher"`
		Repository struct {
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
		} `json:"repository"`
		Commits []struct {
			Message string `json:"message"`
			URL     string `json:"url"`
			Author  struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commits"`
	}
	json.Unmarshal(body, &ev)

	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	repo := ev.Repository.FullName

	var commitLines strings.Builder
	for i, c := range ev.Commits {
		if i >= 10 {
			commitLines.WriteString(fmt.Sprintf("\n... and %d more commits", len(ev.Commits)-10))
			break
		}
		firstLine := strings.SplitN(c.Message, "\n", 2)[0]
		commitLines.WriteString(fmt.Sprintf("- [`%s`](%s) %s\n", firstLine, c.URL, c.Author.Name))
	}

	return map[string]string{
		"ref":         ev.Ref,
		"branch":      branch,
		"repo":        repo,
		"pusher":      ev.Pusher.Name,
		"compare_url": ev.Compare,
		"repo_url":    ev.Repository.HTMLURL,
		"commits":     commitLines.String(),
		"title":       fmt.Sprintf("Push to %s/%s by %s", repo, branch, ev.Pusher.Name),
		"body":        fmt.Sprintf("**Branch:** `%s`\n**Pusher:** %s\n**Compare:** [view diff](%s)\n\n**Commits:**\n%s", branch, ev.Pusher.Name, ev.Compare, commitLines.String()),
	}
}

func parsePullRequestEvent(body []byte) map[string]string {
	var ev struct {
		Action      string `json:"action"`
		PullRequest struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
			Head struct {
				Ref string `json:"ref"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
			Merged bool `json:"merged"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	json.Unmarshal(body, &ev)

	pr := ev.PullRequest
	action := ev.Action
	if action == "closed" && pr.Merged {
		action = "merged"
	}

	prBody := pr.Body
	if len(prBody) > 2000 {
		prBody = prBody[:2000] + "\n\n... (truncated)"
	}

	return map[string]string{
		"action":      action,
		"number":      fmt.Sprintf("%d", pr.Number),
		"title":       pr.Title,
		"user":        pr.User.Login,
		"repo":        ev.Repository.FullName,
		"html_url":    pr.HTMLURL,
		"head_branch": pr.Head.Ref,
		"base_branch": pr.Base.Ref,
		"body":        fmt.Sprintf("**PR [#%d](%s): %s**\n**Author:** %s\n**Branch:** `%s` → `%s`\n**Action:** %s\n\n%s", pr.Number, pr.HTMLURL, pr.Title, pr.User.Login, pr.Head.Ref, pr.Base.Ref, action, prBody),
	}
}

func parseIssuesEvent(body []byte) map[string]string {
	var ev struct {
		Action string `json:"action"`
		Issue  struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		} `json:"issue"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	json.Unmarshal(body, &ev)

	issue := ev.Issue
	var labelNames []string
	for _, l := range issue.Labels {
		labelNames = append(labelNames, l.Name)
	}

	issueBody := issue.Body
	if len(issueBody) > 2000 {
		issueBody = issueBody[:2000] + "\n\n... (truncated)"
	}

	return map[string]string{
		"action":   ev.Action,
		"number":   fmt.Sprintf("%d", issue.Number),
		"title":    issue.Title,
		"user":     issue.User.Login,
		"repo":     ev.Repository.FullName,
		"html_url": issue.HTMLURL,
		"labels":   strings.Join(labelNames, ", "),
		"body":     fmt.Sprintf("**GitHub Issue [#%d](%s): %s**\n**Author:** %s\n**Action:** %s\n\n%s", issue.Number, issue.HTMLURL, issue.Title, issue.User.Login, ev.Action, issueBody),
	}
}

func parseIssueCommentEvent(body []byte) map[string]string {
	var ev struct {
		Action  string `json:"action"`
		Comment struct {
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"comment"`
		Issue struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			HTMLURL string `json:"html_url"`
		} `json:"issue"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	json.Unmarshal(body, &ev)

	commentBody := ev.Comment.Body
	if len(commentBody) > 2000 {
		commentBody = commentBody[:2000] + "\n\n... (truncated)"
	}

	return map[string]string{
		"action":       ev.Action,
		"number":       fmt.Sprintf("%d", ev.Issue.Number),
		"issue_title":  ev.Issue.Title,
		"comment_body": commentBody,
		"user":         ev.Comment.User.Login,
		"repo":         ev.Repository.FullName,
		"html_url":     ev.Comment.HTMLURL,
		"issue_url":    ev.Issue.HTMLURL,
		"title":        fmt.Sprintf("Comment on %s#%d", ev.Repository.FullName, ev.Issue.Number),
		"body":         fmt.Sprintf("**Comment on [%s#%d](%s): %s**\n**By:** %s\n\n%s", ev.Repository.FullName, ev.Issue.Number, ev.Issue.HTMLURL, ev.Issue.Title, ev.Comment.User.Login, commentBody),
	}
}

// --- Default templates ---

func defaultTitleTemplate(eventType string) string {
	switch eventType {
	case "push":
		return "Push to {{.repo}}/{{.branch}} by {{.pusher}}"
	case "pull_request":
		return "PR #{{.number}}: {{.title}} ({{.action}})"
	case "issues":
		return "GitHub Issue #{{.number}}: {{.title}} ({{.action}})"
	case "issue_comment":
		return "Comment on {{.repo}}#{{.number}}"
	default:
		return ""
	}
}

func defaultDescriptionTemplate(_ string) string {
	return ""
}

// --- GitHub Event Rule CRUD ---

type GitHubEventRuleResponse struct {
	ID                  string  `json:"id"`
	WorkspaceID         string  `json:"workspace_id"`
	EventType           string  `json:"event_type"`
	AgentID             string  `json:"agent_id"`
	Enabled             bool    `json:"enabled"`
	TitleTemplate       string  `json:"title_template"`
	DescriptionTemplate string  `json:"description_template"`
	DispatchProvider    *string `json:"dispatch_provider"`
	DispatchDaemonID    *string `json:"dispatch_daemon_id"`
	DispatchDaemonLabel *string `json:"dispatch_daemon_label"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func githubEventRuleToResponse(r db.GithubEventRule) GitHubEventRuleResponse {
	return GitHubEventRuleResponse{
		ID:                  uuidToString(r.ID),
		WorkspaceID:         uuidToString(r.WorkspaceID),
		EventType:           r.EventType,
		AgentID:             uuidToString(r.AgentID),
		Enabled:             r.Enabled,
		TitleTemplate:       r.TitleTemplate,
		DescriptionTemplate: r.DescriptionTemplate,
		DispatchProvider:    textToPtr(r.DispatchProvider),
		DispatchDaemonID:    uuidToPtr(r.DispatchDaemonID),
		DispatchDaemonLabel: textToPtr(r.DispatchDaemonLabel),
		CreatedAt:           timestampToString(r.CreatedAt),
		UpdatedAt:           timestampToString(r.UpdatedAt),
	}
}

func (h *Handler) ListGitHubEventRules(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	rules, err := h.Queries.ListGitHubEventRules(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}

	resp := make([]GitHubEventRuleResponse, len(rules))
	for i, rule := range rules {
		resp[i] = githubEventRuleToResponse(rule)
	}
	writeJSON(w, http.StatusOK, resp)
}

type UpsertGitHubEventRuleRequest struct {
	EventType           string  `json:"event_type"`
	AgentID             string  `json:"agent_id"`
	Enabled             *bool   `json:"enabled"`
	TitleTemplate       string  `json:"title_template"`
	DescriptionTemplate string  `json:"description_template"`
	DispatchProvider    *string `json:"dispatch_provider"`
	DispatchDaemonID    *string `json:"dispatch_daemon_id"`
	DispatchDaemonLabel *string `json:"dispatch_daemon_label"`
}

var validGitHubEventTypes = map[string]bool{
	"push":          true,
	"pull_request":  true,
	"issues":        true,
	"issue_comment": true,
}

func (h *Handler) UpsertGitHubEventRule(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	var req UpsertGitHubEventRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validGitHubEventTypes[req.EventType] {
		writeError(w, http.StatusBadRequest, "invalid event_type (must be push, pull_request, issues, or issue_comment)")
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	params := db.UpsertGitHubEventRuleParams{
		WorkspaceID:         parseUUID(workspaceID),
		EventType:           req.EventType,
		AgentID:             parseUUID(req.AgentID),
		Enabled:             enabled,
		TitleTemplate:       req.TitleTemplate,
		DescriptionTemplate: req.DescriptionTemplate,
		DispatchProvider:    ptrToText(req.DispatchProvider),
		DispatchDaemonID:    parseOptionalUUID(req.DispatchDaemonID),
		DispatchDaemonLabel: ptrToText(req.DispatchDaemonLabel),
	}

	rule, err := h.Queries.UpsertGitHubEventRule(r.Context(), params)
	if err != nil {
		slog.Warn("upsert github event rule failed", "error", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "failed to save rule")
		return
	}

	writeJSON(w, http.StatusOK, githubEventRuleToResponse(rule))
}

func (h *Handler) DeleteGitHubEventRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleId")
	if err := h.Queries.DeleteGitHubEventRule(r.Context(), parseUUID(ruleID)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
