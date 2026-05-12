package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
	"github.com/nullne/multica/server/pkg/redact"
)

// ---------------------------------------------------------------------------
// Daemon Registration & Heartbeat
// ---------------------------------------------------------------------------

type DaemonRegisterRuntime struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Version    string `json:"version"` // agent CLI version (claude/codex)
	Status     string `json:"status"`
	AuthStatus string `json:"auth_status"`
}

type DaemonRegisterRequest struct {
	DaemonID   string                  `json:"daemon_id"`
	DeviceName string                  `json:"device_name"`
	CLIVersion string                  `json:"cli_version"` // multica CLI version
	EnvVars    map[string]string       `json:"env_vars,omitempty"`
	Runtimes   []DaemonRegisterRuntime `json:"runtimes"`
}

// WorkspaceRegistration describes a daemon's projection into one workspace.
type WorkspaceRegistration struct {
	WorkspaceID    string                    `json:"workspace_id"`
	WorkspaceName  string                    `json:"workspace_name"`
	Enabled        bool                      `json:"enabled"`
	Runtimes       []AgentRuntimeResponse    `json:"runtimes"`
	Repos          []RepoData                `json:"repos"`
	ProviderConfig map[string]map[string]any `json:"provider_config,omitempty"`
}

func (h *Handler) DaemonRegister(w http.ResponseWriter, r *http.Request) {
	var req DaemonRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.DaemonID = strings.TrimSpace(req.DaemonID)
	req.DeviceName = strings.TrimSpace(req.DeviceName)

	if req.DaemonID == "" {
		writeError(w, http.StatusBadRequest, "daemon_id is required")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	// Upsert the daemon entity scoped to the authenticated user.
	meta := map[string]any{"cli_version": req.CLIVersion}
	if len(req.EnvVars) > 0 {
		meta["env_vars"] = req.EnvVars
	}
	daemonMeta, _ := json.Marshal(meta)
	daemon, err := h.Queries.UpsertDaemon(r.Context(), db.UpsertDaemonParams{
		UserID:     parseUUID(userID),
		DaemonID:   req.DaemonID,
		Status:     "online",
		CliVersion: req.CLIVersion,
		DeviceName: req.DeviceName,
		DeviceInfo: req.DeviceName,
		Metadata:   daemonMeta,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register daemon: "+err.Error())
		return
	}

	// Decide which workspaces this daemon serves. On the very first
	// registration we auto-enable every workspace the user is a member of so
	// the daemon is immediately useful — subsequent registrations preserve
	// whatever the user has configured.
	existing, _ := h.Queries.ListWorkspacesForDaemon(r.Context(), daemon.ID)
	if len(existing) == 0 {
		memberWs, _ := h.Queries.ListWorkspaces(r.Context(), parseUUID(userID))
		for _, ws := range memberWs {
			if _, err := h.Queries.UpsertDaemonWorkspace(r.Context(), db.UpsertDaemonWorkspaceParams{
				DaemonID:    daemon.ID,
				WorkspaceID: ws.ID,
				Enabled:     true,
			}); err != nil {
				slog.Warn("auto-assign daemon to workspace failed", "daemon_id", uuidToString(daemon.ID), "workspace_id", uuidToString(ws.ID), "error", err)
			}
		}
	}

	memberWorkspaces, err := h.Queries.ListWorkspaces(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspaces")
		return
	}
	enabledWorkspaceIDs, err := h.Queries.ListEnabledWorkspacesForDaemon(r.Context(), daemon.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace assignments")
		return
	}
	enabledByWorkspace := make(map[string]struct{}, len(enabledWorkspaceIDs))
	for _, wsID := range enabledWorkspaceIDs {
		enabledByWorkspace[uuidToString(wsID)] = struct{}{}
	}

	mergedProviderConfig := make(map[string]map[string]any)
	mergeProviderConfig := func(ps WorkspaceProviderSettings) {
		if ps.Providers == nil {
			return
		}
		for k, v := range ps.Providers {
			cur := mergedProviderConfig[k]
			if cur == nil {
				cur = map[string]any{
					"enabled":        v.Enabled,
					"api_key":        v.APIKey,
					"target_version": v.TargetVersion,
				}
				mergedProviderConfig[k] = cur
				continue
			}
			// Provider is "enabled across daemon" if any workspace enables it.
			if v.Enabled {
				cur["enabled"] = true
			}
			if cur["api_key"] == "" && v.APIKey != "" {
				cur["api_key"] = v.APIKey
			}
			if cur["target_version"] == "" && v.TargetVersion != "" {
				cur["target_version"] = v.TargetVersion
			}
		}
	}

	workspaceRegs := make([]WorkspaceRegistration, 0, len(memberWorkspaces))
	var multicaTargetVersion string

	for _, ws := range memberWorkspaces {
		wsID := ws.ID
		_, enabled := enabledByWorkspace[uuidToString(wsID)]
		ps := parseProviderSettings(ws.Settings)
		if multicaTargetVersion == "" && ps.MulticaTargetVersion != "" {
			multicaTargetVersion = ps.MulticaTargetVersion
		}
		mergeProviderConfig(ps)

		runtimeResp, err := h.projectDaemonRuntimes(r.Context(), daemon, req, ps, wsID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to register runtime: "+err.Error())
			return
		}

		var repos []RepoData
		if ws.Repos != nil {
			json.Unmarshal(ws.Repos, &repos)
		}
		if repos == nil {
			repos = []RepoData{}
		}

		wsProviderConfig := make(map[string]map[string]any)
		if ps.Providers != nil {
			for k, v := range ps.Providers {
				wsProviderConfig[k] = map[string]any{
					"enabled":        v.Enabled,
					"api_key":        v.APIKey,
					"target_version": v.TargetVersion,
				}
			}
		}

		workspaceRegs = append(workspaceRegs, WorkspaceRegistration{
			WorkspaceID:    uuidToString(wsID),
			WorkspaceName:  ws.Name,
			Enabled:        enabled,
			Runtimes:       runtimeResp,
			Repos:          repos,
			ProviderConfig: wsProviderConfig,
		})

		if enabled {
			h.publish(protocol.EventDaemonRegister, uuidToString(wsID), "system", "", map[string]any{
				"runtimes": runtimeResp,
			})
		}
	}

	slog.Info("daemon registered",
		"user_id", userID,
		"daemon_id", req.DaemonID,
		"daemon_uuid", uuidToString(daemon.ID),
		"workspaces", len(workspaceRegs),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"daemon":                 daemonToResponse(daemon),
		"workspaces":             workspaceRegs,
		"provider_config":        mergedProviderConfig,
		"multica_target_version": multicaTargetVersion,
	})
}

// projectDaemonRuntimes upserts the per-workspace runtime rows for a daemon
// based on the locally detected runtime payload.
func (h *Handler) projectDaemonRuntimes(ctx context.Context, daemon db.Daemon, req DaemonRegisterRequest, ps WorkspaceProviderSettings, workspaceID pgtype.UUID) ([]AgentRuntimeResponse, error) {
	out := make([]AgentRuntimeResponse, 0, len(req.Runtimes))
	for _, runtime := range req.Runtimes {
		provider := strings.TrimSpace(runtime.Type)
		if provider == "" {
			provider = "unknown"
		}
		name := strings.TrimSpace(runtime.Name)
		if name == "" {
			name = provider
			if req.DeviceName != "" {
				name = fmt.Sprintf("%s (%s)", provider, req.DeviceName)
			}
		}
		deviceInfo := strings.TrimSpace(req.DeviceName)
		if runtime.Version != "" && deviceInfo != "" {
			deviceInfo = fmt.Sprintf("%s · %s", deviceInfo, runtime.Version)
		} else if runtime.Version != "" {
			deviceInfo = runtime.Version
		}
		status := "online"
		if runtime.Status == "offline" {
			status = "offline"
		}
		authStatus := "unknown"
		switch runtime.AuthStatus {
		case "not_installed", "unauthenticated", "ready":
			authStatus = runtime.AuthStatus
		}
		metadata, _ := json.Marshal(map[string]any{
			"version":     runtime.Version,
			"cli_version": req.CLIVersion,
		})

		registered, err := h.Queries.UpsertAgentRuntime(ctx, db.UpsertAgentRuntimeParams{
			WorkspaceID: workspaceID,
			DaemonID:    strToText(daemon.DaemonID),
			DaemonRef:   daemon.ID,
			Name:        name,
			RuntimeMode: "local",
			Provider:    provider,
			Status:      status,
			AuthStatus:  authStatus,
			DeviceInfo:  deviceInfo,
			Metadata:    metadata,
		})
		if err != nil {
			return nil, err
		}
		rr := runtimeToResponse(registered)
		rr.AuthStatus = effectiveAuthStatus(rr.AuthStatus, provider, ps)
		out = append(out, rr)
	}
	return out, nil
}

// DaemonDeregister marks a daemon and all its runtimes as offline.
func (h *Handler) DaemonDeregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DaemonID string `json:"daemon_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DaemonID == "" {
		writeError(w, http.StatusBadRequest, "daemon_id is required")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	d, err := h.Queries.GetDaemonForUser(r.Context(), db.GetDaemonForUserParams{
		ID:     parseUUID(req.DaemonID),
		UserID: parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "daemon not found")
		return
	}

	if err := h.Queries.SetDaemonAndRuntimesOffline(r.Context(), d.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deregister daemon")
		return
	}
	slog.Info("daemon deregistered", "daemon_id", req.DaemonID, "user_id", userID)

	// Broadcast to every workspace this daemon was visible in so frontends
	// can refresh their runtime lists.
	if assignments, err := h.Queries.ListWorkspacesForDaemon(r.Context(), d.ID); err == nil {
		for _, a := range assignments {
			h.publish(protocol.EventDaemonRegister, uuidToString(a.WorkspaceID), "system", "", map[string]any{
				"action": "deregister",
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type DaemonHeartbeatRequest struct {
	DaemonID     string            `json:"daemon_id"`
	AuthStatuses map[string]string `json:"auth_statuses,omitempty"` // provider -> auth_status from periodic re-check
}

func (h *Handler) DaemonHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req DaemonHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DaemonID == "" {
		writeError(w, http.StatusBadRequest, "daemon_id is required")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	daemon, err := h.Queries.GetDaemonForUser(r.Context(), db.GetDaemonForUserParams{
		ID:     parseUUID(req.DaemonID),
		UserID: parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "daemon not found")
		return
	}

	if _, err := h.Queries.UpdateDaemonHeartbeat(r.Context(), daemon.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "heartbeat failed")
		return
	}
	h.Queries.UpdateRuntimesHeartbeatByDaemon(r.Context(), daemon.ID)

	// Update per-provider auth_status if reported.
	if len(req.AuthStatuses) > 0 {
		for provider, authStatus := range req.AuthStatuses {
			switch authStatus {
			case "not_installed", "unauthenticated", "ready":
				h.Queries.UpdateRuntimesAuthStatusByDaemon(r.Context(), db.UpdateRuntimesAuthStatusByDaemonParams{
					DaemonRef:  daemon.ID,
					AuthStatus: authStatus,
					Provider:   provider,
				})
			}
		}
	}

	slog.Debug("daemon heartbeat", "daemon_id", req.DaemonID)

	resp := map[string]any{"status": "ok"}

	if pending := h.PingStore.PopPending(req.DaemonID); pending != nil {
		resp["pending_ping"] = map[string]string{"id": pending.ID}
	}
	if updates := h.UpdateStore.PopAllPending(req.DaemonID); len(updates) > 0 {
		out := make([]map[string]string, len(updates))
		for i, u := range updates {
			out[i] = map[string]string{
				"id":             u.ID,
				"target":         u.Target,
				"target_version": u.TargetVersion,
			}
		}
		resp["pending_updates"] = out
	}

	// Project the union of provider configs across enabled workspaces so the
	// daemon can auto-install any provider any of its workspaces wants.
	merged := make(map[string]map[string]any)
	if enabled, err := h.Queries.ListEnabledWorkspacesForDaemon(r.Context(), daemon.ID); err == nil {
		for _, wsID := range enabled {
			ws, err := h.Queries.GetWorkspace(r.Context(), wsID)
			if err != nil {
				continue
			}
			ps := parseProviderSettings(ws.Settings)
			for k, v := range ps.Providers {
				cur := merged[k]
				if cur == nil {
					cur = map[string]any{
						"enabled":        v.Enabled,
						"api_key":        v.APIKey,
						"target_version": v.TargetVersion,
					}
					merged[k] = cur
					continue
				}
				if v.Enabled {
					cur["enabled"] = true
				}
				if cur["api_key"] == "" && v.APIKey != "" {
					cur["api_key"] = v.APIKey
				}
				if cur["target_version"] == "" && v.TargetVersion != "" {
					cur["target_version"] = v.TargetVersion
				}
			}
		}
	}
	if len(merged) > 0 {
		resp["provider_config"] = merged
	}

	writeJSON(w, http.StatusOK, resp)
}

// ClaimTaskByRuntime atomically claims the next queued task for a runtime.
// The response includes the agent's name and skills, fetched fresh from the DB.
func (h *Handler) ClaimTaskByRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")

	task, err := h.TaskService.ClaimTaskForRuntime(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to claim task: "+err.Error())
		return
	}

	if task == nil {
		slog.Debug("no task to claim", "runtime_id", runtimeID)
		writeJSON(w, http.StatusOK, map[string]any{"task": nil})
		return
	}

	// Build response with fresh agent data (name + skills).
	resp := taskToResponse(*task)
	var agentCodeAccess string
	if agent, err := h.Queries.GetAgent(r.Context(), task.AgentID); err == nil {
		skills := h.TaskService.LoadAgentSkills(r.Context(), task.AgentID)
		resp.Agent = &TaskAgentData{
			ID:           uuidToString(agent.ID),
			Name:         agent.Name,
			Instructions: agent.Instructions,
			Skills:       skills,
		}
		agentCodeAccess = agent.GithubCodeAccess
	}

	// Look up the runtime to get the provider for API key injection.
	var runtimeProvider string
	if rt, err := h.Queries.GetAgentRuntime(r.Context(), task.RuntimeID); err == nil {
		runtimeProvider = rt.Provider
	}

	// Include workspace ID and repos so the daemon can set up worktrees.
	// Also generate a scoped GitHub token if a GitHub App is configured.
	if issue, err := h.Queries.GetIssue(r.Context(), task.IssueID); err == nil {
		resp.WorkspaceID = uuidToString(issue.WorkspaceID)
		if ws, err := h.Queries.GetWorkspace(r.Context(), issue.WorkspaceID); err == nil {
			if ws.Repos != nil {
				var repos []RepoData
				if json.Unmarshal(ws.Repos, &repos) == nil && len(repos) > 0 {
					resp.Repos = repos
				}
			}
			// Generate scoped GitHub installation token for the agent.
			if agentCodeAccess != "" {
				var repoURLs []string
				for _, r := range resp.Repos {
					repoURLs = append(repoURLs, r.URL)
				}
				if token := h.generateGitHubTokenForAgent(r, ws, agentCodeAccess, repoURLs); token != "" {
					resp.GitHubToken = token
					resp.GitHubCodeAccess = agentCodeAccess
				}
			}
			// Inject workspace-level provider API key for the runtime's provider.
			if runtimeProvider != "" {
				ps := parseProviderSettings(ws.Settings)
				if pc, ok := ps.Providers[runtimeProvider]; ok && pc.APIKey != "" {
					resp.ProviderAPIKey = pc.APIKey
				}
			}
		}
	}

	// Look up the prior session for this (agent, issue) pair so the daemon
	// can resume the Claude Code conversation context.
	if prior, err := h.Queries.GetLastTaskSession(r.Context(), db.GetLastTaskSessionParams{
		AgentID: task.AgentID,
		IssueID: task.IssueID,
	}); err == nil && prior.SessionID.Valid {
		resp.PriorSessionID = prior.SessionID.String
		if prior.WorkDir.Valid {
			resp.PriorWorkDir = prior.WorkDir.String
		}
	}

	slog.Info("task claimed by runtime", "task_id", uuidToString(task.ID), "runtime_id", runtimeID, "agent_id", uuidToString(task.AgentID), "prior_session", resp.PriorSessionID)
	writeJSON(w, http.StatusOK, map[string]any{"task": resp})
}

// ListPendingTasksByRuntime returns queued/dispatched tasks for a runtime.
func (h *Handler) ListPendingTasksByRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")

	tasks, err := h.Queries.ListPendingTasksByRuntime(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pending tasks")
		return
	}

	resp := make([]AgentTaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = taskToResponse(t)
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Task Lifecycle (called by daemon)
// ---------------------------------------------------------------------------

// StartTask marks a dispatched task as running.
func (h *Handler) StartTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	task, err := h.TaskService.StartTask(r.Context(), parseUUID(taskID))
	if err != nil {
		slog.Warn("start task failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("task started", "task_id", taskID, "agent_id", uuidToString(task.AgentID))
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

// ReportTaskProgress broadcasts a progress update.
type TaskProgressRequest struct {
	Summary string `json:"summary"`
	Step    int    `json:"step"`
	Total   int    `json:"total"`
}

func (h *Handler) ReportTaskProgress(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Look up task to get workspace ID via the associated issue.
	workspaceID := ""
	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err == nil {
		if issue, err := h.Queries.GetIssue(r.Context(), task.IssueID); err == nil {
			workspaceID = uuidToString(issue.WorkspaceID)
		}
	}

	h.TaskService.ReportProgress(r.Context(), taskID, workspaceID, req.Summary, req.Step, req.Total)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CompleteTask marks a running task as completed.
type TaskCompleteRequest struct {
	PRURL      string `json:"pr_url"`
	BranchName string `json:"branch_name"`
	Output     string `json:"output"`
	SessionID  string `json:"session_id"` // Claude session ID for future resumption
	WorkDir    string `json:"work_dir"`   // working directory used during execution
}

func (h *Handler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, _ := json.Marshal(req)
	task, err := h.TaskService.CompleteTask(r.Context(), parseUUID(taskID), result, req.SessionID, req.WorkDir)
	if err != nil {
		slog.Warn("complete task failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("task completed", "task_id", taskID, "agent_id", uuidToString(task.AgentID))
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

// GetTaskStatus returns the current status of a task.
// Used by the daemon to check whether a task was cancelled mid-execution.
func (h *Handler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": task.Status})
}

// FailTask marks a running task as failed.
type TaskFailRequest struct {
	Error string `json:"error"`
}

func (h *Handler) FailTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskFailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	task, err := h.TaskService.FailTask(r.Context(), parseUUID(taskID), req.Error)
	if err != nil {
		slog.Warn("fail task failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("task failed", "task_id", taskID, "agent_id", uuidToString(task.AgentID), "task_error", req.Error)
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

// ---------------------------------------------------------------------------
// Task Messages (live agent output)
// ---------------------------------------------------------------------------

type TaskMessageRequest struct {
	Seq     int            `json:"seq"`
	Type    string         `json:"type"`
	Tool    string         `json:"tool,omitempty"`
	Content string         `json:"content,omitempty"`
	Input   map[string]any `json:"input,omitempty"`
	Output  string         `json:"output,omitempty"`
}

type TaskMessageBatchRequest struct {
	Messages []TaskMessageRequest `json:"messages"`
}

// ReportTaskMessages receives a batch of agent execution messages from the daemon.
func (h *Handler) ReportTaskMessages(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskMessageBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Messages) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	workspaceID := ""
	if issue, err := h.Queries.GetIssue(r.Context(), task.IssueID); err == nil {
		workspaceID = uuidToString(issue.WorkspaceID)
	}

	for _, msg := range req.Messages {
		// Redact sensitive information before persisting or broadcasting.
		msg.Content = redact.Text(msg.Content)
		msg.Output = redact.Text(msg.Output)
		msg.Input = redact.InputMap(msg.Input)

		var inputJSON []byte
		if msg.Input != nil {
			inputJSON, _ = json.Marshal(msg.Input)
		}
		h.Queries.CreateTaskMessage(r.Context(), db.CreateTaskMessageParams{
			TaskID:  parseUUID(taskID),
			Seq:     int32(msg.Seq),
			Type:    msg.Type,
			Tool:    pgtype.Text{String: msg.Tool, Valid: msg.Tool != ""},
			Content: pgtype.Text{String: msg.Content, Valid: msg.Content != ""},
			Input:   inputJSON,
			Output:  pgtype.Text{String: msg.Output, Valid: msg.Output != ""},
		})

		if workspaceID != "" {
			h.publish(protocol.EventTaskMessage, workspaceID, "system", "", protocol.TaskMessagePayload{
				TaskID:  taskID,
				IssueID: uuidToString(task.IssueID),
				Seq:     msg.Seq,
				Type:    msg.Type,
				Tool:    msg.Tool,
				Content: msg.Content,
				Input:   msg.Input,
				Output:  msg.Output,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListTaskMessages returns the persisted messages for a task (for catch-up after reconnect).
func (h *Handler) ListTaskMessages(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	var messages []db.TaskMessage
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		sinceSeq, parseErr := strconv.Atoi(sinceStr)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid since parameter")
			return
		}
		messages, err = h.Queries.ListTaskMessagesSince(r.Context(), db.ListTaskMessagesSinceParams{
			TaskID: parseUUID(taskID),
			Seq:    int32(sinceSeq),
		})
	} else {
		messages, err = h.Queries.ListTaskMessages(r.Context(), parseUUID(taskID))
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list task messages")
		return
	}

	issueID := uuidToString(task.IssueID)

	resp := make([]protocol.TaskMessagePayload, len(messages))
	for i, m := range messages {
		var input map[string]any
		if m.Input != nil {
			json.Unmarshal(m.Input, &input)
		}
		resp[i] = protocol.TaskMessagePayload{
			TaskID:  taskID,
			IssueID: issueID,
			Seq:     int(m.Seq),
			Type:    m.Type,
			Tool:    m.Tool.String,
			Content: m.Content.String,
			Input:   input,
			Output:  m.Output.String,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetActiveTaskForIssue returns the currently running task for an issue, if any.
func (h *Handler) GetActiveTaskForIssue(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")

	tasks, err := h.Queries.ListActiveTasksByIssue(r.Context(), parseUUID(issueID))
	if err != nil || len(tasks) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"task": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"task": taskToResponse(tasks[0])})
}

// CancelTask cancels a running or queued task by ID.
func (h *Handler) CancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	task, err := h.TaskService.CancelTask(r.Context(), parseUUID(taskID))
	if err != nil {
		slog.Warn("cancel task failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("task cancelled by user", "task_id", taskID, "issue_id", uuidToString(task.IssueID))
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

// ListActiveTasksForWorkspace returns all active (queued/dispatched/running) tasks for the workspace.
func (h *Handler) ListActiveTasksForWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	tasks, err := h.Queries.ListActiveTasksByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list active tasks")
		return
	}

	resp := make([]AgentTaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = taskToResponse(t)
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListTasksByIssue returns all tasks (any status) for an issue — used for execution history.
func (h *Handler) ListTasksByIssue(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")

	tasks, err := h.Queries.ListTasksByIssue(r.Context(), parseUUID(issueID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	resp := make([]AgentTaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = taskToResponse(t)
	}

	writeJSON(w, http.StatusOK, resp)
}
