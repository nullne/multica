package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	gh "github.com/nullne/multica/server/internal/github"
	"github.com/nullne/multica/server/internal/logger"
	"github.com/nullne/multica/server/internal/service"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

type AgentResponse struct {
	ID                 string          `json:"id"`
	WorkspaceID        string          `json:"workspace_id"`
	Providers          []string        `json:"providers"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Instructions       string          `json:"instructions"`
	AvatarURL          *string         `json:"avatar_url"`
	Visibility         string          `json:"visibility"`
	Status             string          `json:"status"`
	OwnerID            *string         `json:"owner_id"`
	Skills             []SkillResponse `json:"skills"`
	Tools              any             `json:"tools"`
	Triggers           any             `json:"triggers"`
	GitHubCodeAccess   string          `json:"github_code_access"`
	DefaultDaemonID    *string         `json:"default_daemon_id"`
	MaxConcurrentTasks int32           `json:"max_concurrent_tasks"`
	CreatedAt          string          `json:"created_at"`
	UpdatedAt          string          `json:"updated_at"`
	ArchivedAt         *string         `json:"archived_at"`
	ArchivedBy         *string         `json:"archived_by"`
}

func agentToResponse(a db.Agent) AgentResponse {
	var tools any
	if a.Tools != nil {
		json.Unmarshal(a.Tools, &tools)
	}
	if tools == nil {
		tools = []any{}
	}

	var triggers any
	if a.Triggers != nil {
		json.Unmarshal(a.Triggers, &triggers)
	}
	if triggers == nil {
		triggers = []any{}
	}

	providers := a.Providers
	if providers == nil {
		providers = []string{}
	}

	return AgentResponse{
		ID:                 uuidToString(a.ID),
		WorkspaceID:        uuidToString(a.WorkspaceID),
		Providers:          providers,
		Name:               a.Name,
		Description:        a.Description,
		Instructions:       a.Instructions,
		AvatarURL:          textToPtr(a.AvatarUrl),
		Visibility:         a.Visibility,
		Status:             a.Status,
		OwnerID:            uuidToPtr(a.OwnerID),
		Skills:             []SkillResponse{},
		Tools:              tools,
		Triggers:           triggers,
		GitHubCodeAccess:   a.GithubCodeAccess,
		DefaultDaemonID:    uuidToPtr(a.DefaultDaemonID),
		MaxConcurrentTasks: a.MaxConcurrentTasks,
		CreatedAt:          timestampToString(a.CreatedAt),
		UpdatedAt:          timestampToString(a.UpdatedAt),
		ArchivedAt:         timestampToPtr(a.ArchivedAt),
		ArchivedBy:         uuidToPtr(a.ArchivedBy),
	}
}

// RepoData holds repository information included in claim responses so the
// daemon can set up worktrees for each workspace repo.
type RepoData struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type AgentTaskResponse struct {
	ID               string         `json:"id"`
	AgentID          string         `json:"agent_id"`
	RuntimeID        string         `json:"runtime_id"`
	IssueID          string         `json:"issue_id"`
	WorkspaceID      string         `json:"workspace_id"`
	Status           string         `json:"status"`
	Priority         int32          `json:"priority"`
	DispatchedAt     *string        `json:"dispatched_at"`
	StartedAt        *string        `json:"started_at"`
	CompletedAt      *string        `json:"completed_at"`
	Result           any            `json:"result"`
	Context          map[string]any `json:"context,omitempty"`
	Error            *string        `json:"error"`
	Agent            *TaskAgentData `json:"agent,omitempty"`
	Repos            []RepoData     `json:"repos,omitempty"`
	CreatedAt        string         `json:"created_at"`
	PriorSessionID   string         `json:"prior_session_id,omitempty"`   // session ID from a previous task on same issue
	PriorWorkDir     string         `json:"prior_work_dir,omitempty"`     // work_dir from a previous task on same issue
	TriggerCommentID *string        `json:"trigger_comment_id,omitempty"` // comment that triggered this task
	GitHubToken      string `json:"github_token,omitempty"`
	GitHubCodeAccess string `json:"github_code_access,omitempty"`
	ProviderAPIKey   string `json:"provider_api_key,omitempty"` // workspace-level API key for the provider
}

// TaskAgentData holds agent info included in claim responses so the daemon
// can set up the execution environment (branch naming, skill files, instructions).
type TaskAgentData struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	Instructions string                   `json:"instructions"`
	Skills       []service.AgentSkillData `json:"skills,omitempty"`
}

func taskToResponse(t db.AgentTaskQueue) AgentTaskResponse {
	var result any
	if t.Result != nil {
		json.Unmarshal(t.Result, &result)
	}
	var contextData map[string]any
	if t.Context != nil {
		json.Unmarshal(t.Context, &contextData)
	}
	return AgentTaskResponse{
		ID:               uuidToString(t.ID),
		AgentID:          uuidToString(t.AgentID),
		RuntimeID:        uuidToString(t.RuntimeID),
		IssueID:          uuidToString(t.IssueID),
		Status:           t.Status,
		Priority:         t.Priority,
		DispatchedAt:     timestampToPtr(t.DispatchedAt),
		StartedAt:        timestampToPtr(t.StartedAt),
		CompletedAt:      timestampToPtr(t.CompletedAt),
		Result:           result,
		Context:          contextData,
		Error:            textToPtr(t.Error),
		CreatedAt:        timestampToString(t.CreatedAt),
		TriggerCommentID: uuidToPtr(t.TriggerCommentID),
	}
}

func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	var agents []db.Agent
	var err error
	if r.URL.Query().Get("include_archived") == "true" {
		agents, err = h.Queries.ListAllAgents(r.Context(), parseUUID(workspaceID))
	} else {
		agents, err = h.Queries.ListAgents(r.Context(), parseUUID(workspaceID))
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}

	// Batch-load skills for all agents to avoid N+1.
	skillRows, err := h.Queries.ListAgentSkillsByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load agent skills")
		return
	}
	skillMap := map[string][]SkillResponse{}
	for _, row := range skillRows {
		agentID := uuidToString(row.AgentID)
		skillMap[agentID] = append(skillMap[agentID], SkillResponse{
			ID:          uuidToString(row.ID),
			Name:        row.Name,
			Description: row.Description,
		})
	}

	// All agents (including private) are visible to workspace members.
	visible := make([]AgentResponse, 0, len(agents))
	for _, a := range agents {
		resp := agentToResponse(a)
		if skills, ok := skillMap[resp.ID]; ok {
			resp.Skills = skills
		}
		visible = append(visible, resp)
	}

	writeJSON(w, http.StatusOK, visible)
}

func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := h.loadAgentForUser(w, r, id)
	if !ok {
		return
	}
	resp := agentToResponse(agent)
	skills, err := h.Queries.ListAgentSkills(r.Context(), agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load agent skills")
		return
	}
	if len(skills) > 0 {
		resp.Skills = make([]SkillResponse, len(skills))
		for i, s := range skills {
			resp.Skills[i] = skillToResponse(s)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type CreateAgentRequest struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Instructions       string   `json:"instructions"`
	AvatarURL          *string  `json:"avatar_url"`
	Providers          []string `json:"providers"`
	Provider           string   `json:"provider"` // deprecated: single provider, use providers
	Visibility         string   `json:"visibility"`
	Tools              any      `json:"tools"`
	Triggers           any      `json:"triggers"`
	GitHubCodeAccess   string   `json:"github_code_access"`
	DefaultDaemonID    *string  `json:"default_daemon_id"`
	MaxConcurrentTasks *int32   `json:"max_concurrent_tasks"`
}

func (h *Handler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ownerID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	providers := req.Providers
	if len(providers) == 0 && req.Provider != "" {
		providers = []string{req.Provider}
	}
	if len(providers) == 0 {
		writeError(w, http.StatusBadRequest, "providers is required")
		return
	}

	if req.Visibility == "" {
		req.Visibility = "private"
	}
	if req.GitHubCodeAccess == "" {
		req.GitHubCodeAccess = "read"
	}
	if !gh.ValidCodeAccess(req.GitHubCodeAccess) {
		writeError(w, http.StatusBadRequest, "invalid github_code_access (must be read, write, or admin)")
		return
	}

	tools, _ := json.Marshal(req.Tools)
	if req.Tools == nil {
		tools = []byte("[]")
	}

	triggers, _ := json.Marshal(req.Triggers)
	if req.Triggers == nil {
		triggers = defaultAgentTriggers()
	}

	maxConcurrentTasks := int32(6)
	if req.MaxConcurrentTasks != nil && *req.MaxConcurrentTasks > 0 {
		maxConcurrentTasks = *req.MaxConcurrentTasks
	}

	agent, err := h.Queries.CreateAgent(r.Context(), db.CreateAgentParams{
		WorkspaceID:        parseUUID(workspaceID),
		Name:               req.Name,
		Description:        req.Description,
		Instructions:       req.Instructions,
		AvatarUrl:          ptrToText(req.AvatarURL),
		Providers:          providers,
		Visibility:         req.Visibility,
		OwnerID:            parseUUID(ownerID),
		Tools:              tools,
		Triggers:           triggers,
		GithubCodeAccess:   req.GitHubCodeAccess,
		DefaultDaemonID:    parseOptionalUUID(req.DefaultDaemonID),
		MaxConcurrentTasks: maxConcurrentTasks,
	})
	if err != nil {
		slog.Warn("create agent failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to create agent: "+err.Error())
		return
	}
	slog.Info("agent created", append(logger.RequestAttrs(r), "agent_id", uuidToString(agent.ID), "name", agent.Name, "workspace_id", workspaceID, "providers", providers)...)

	h.TaskService.ReconcileAgentStatus(r.Context(), agent.ID)
	agent, _ = h.Queries.GetAgent(r.Context(), agent.ID)

	resp := agentToResponse(agent)
	actorType, actorID := h.resolveActor(r, ownerID, workspaceID)
	h.publish(protocol.EventAgentCreated, workspaceID, actorType, actorID, map[string]any{"agent": resp})
	writeJSON(w, http.StatusCreated, resp)
}

type UpdateAgentRequest struct {
	Name               *string  `json:"name"`
	Description        *string  `json:"description"`
	Instructions       *string  `json:"instructions"`
	AvatarURL          *string  `json:"avatar_url"`
	Providers          []string `json:"providers"`
	Visibility         *string  `json:"visibility"`
	Status             *string  `json:"status"`
	Tools              any      `json:"tools"`
	Triggers           any      `json:"triggers"`
	GitHubCodeAccess   *string  `json:"github_code_access"`
	DefaultDaemonID    *string  `json:"default_daemon_id"`
	MaxConcurrentTasks *int32   `json:"max_concurrent_tasks"`
}

// canManageAgent checks whether the current user can update or archive an agent.
// Only the agent owner or workspace owner/admin can manage any agent,
// regardless of whether it is public or private.
func (h *Handler) canManageAgent(w http.ResponseWriter, r *http.Request, agent db.Agent) bool {
	wsID := uuidToString(agent.WorkspaceID)
	member, ok := h.requireWorkspaceRole(w, r, wsID, "agent not found", "owner", "admin", "member")
	if !ok {
		return false
	}
	isAdmin := roleAllowed(member.Role, "owner", "admin")
	isAgentOwner := uuidToString(agent.OwnerID) == requestUserID(r)
	if !isAdmin && !isAgentOwner {
		writeError(w, http.StatusForbidden, "only the agent owner can manage this agent")
		return false
	}
	return true
}

func (h *Handler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := h.loadAgentForUser(w, r, id)
	if !ok {
		return
	}
	if !h.canManageAgent(w, r, agent) {
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req UpdateAgentRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var rawFields map[string]json.RawMessage
	json.Unmarshal(bodyBytes, &rawFields)

	params := db.UpdateAgentParams{
		ID:              parseUUID(id),
		DefaultDaemonID: agent.DefaultDaemonID,
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Instructions != nil {
		params.Instructions = pgtype.Text{String: *req.Instructions, Valid: true}
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = pgtype.Text{String: *req.AvatarURL, Valid: true}
	}
	if len(req.Providers) > 0 {
		params.Providers = req.Providers
	}
	if req.Visibility != nil {
		params.Visibility = pgtype.Text{String: *req.Visibility, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.Tools != nil {
		tools, _ := json.Marshal(req.Tools)
		params.Tools = tools
	}
	if req.Triggers != nil {
		triggers, _ := json.Marshal(req.Triggers)
		params.Triggers = triggers
	}
	if req.GitHubCodeAccess != nil {
		if !gh.ValidCodeAccess(*req.GitHubCodeAccess) {
			writeError(w, http.StatusBadRequest, "invalid github_code_access (must be read, write, or admin)")
			return
		}
		params.GithubCodeAccess = pgtype.Text{String: *req.GitHubCodeAccess, Valid: true}
	}
	if _, ok := rawFields["default_daemon_id"]; ok {
		if req.DefaultDaemonID != nil && *req.DefaultDaemonID != "" {
			params.DefaultDaemonID = parseUUID(*req.DefaultDaemonID)
		} else {
			params.DefaultDaemonID = pgtype.UUID{Valid: false}
		}
	}
	if req.MaxConcurrentTasks != nil {
		if *req.MaxConcurrentTasks <= 0 {
			writeError(w, http.StatusBadRequest, "max_concurrent_tasks must be a positive integer")
			return
		}
		params.MaxConcurrentTasks = pgtype.Int4{Int32: *req.MaxConcurrentTasks, Valid: true}
	}

	agent, err = h.Queries.UpdateAgent(r.Context(), params)
	if err != nil {
		slog.Warn("update agent failed", append(logger.RequestAttrs(r), "error", err, "agent_id", id)...)
		writeError(w, http.StatusInternalServerError, "failed to update agent: "+err.Error())
		return
	}

	resp := agentToResponse(agent)
	slog.Info("agent updated", append(logger.RequestAttrs(r), "agent_id", id, "workspace_id", uuidToString(agent.WorkspaceID))...)
	userID := requestUserID(r)
	actorType, actorID := h.resolveActor(r, userID, uuidToString(agent.WorkspaceID))
	h.publish(protocol.EventAgentStatus, uuidToString(agent.WorkspaceID), actorType, actorID, map[string]any{"agent": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ArchiveAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := h.loadAgentForUser(w, r, id)
	if !ok {
		return
	}
	if !h.canManageAgent(w, r, agent) {
		return
	}
	if agent.ArchivedAt.Valid {
		writeError(w, http.StatusConflict, "agent is already archived")
		return
	}

	userID := requestUserID(r)
	archived, err := h.Queries.ArchiveAgent(r.Context(), db.ArchiveAgentParams{
		ID:         parseUUID(id),
		ArchivedBy: parseUUID(userID),
	})
	if err != nil {
		slog.Warn("archive agent failed", append(logger.RequestAttrs(r), "error", err, "agent_id", id)...)
		writeError(w, http.StatusInternalServerError, "failed to archive agent")
		return
	}

	// Cancel all pending/active tasks for this agent.
	if err := h.Queries.CancelAgentTasksByAgent(r.Context(), parseUUID(id)); err != nil {
		slog.Warn("cancel agent tasks on archive failed", append(logger.RequestAttrs(r), "error", err, "agent_id", id)...)
	}

	wsID := uuidToString(archived.WorkspaceID)
	slog.Info("agent archived", append(logger.RequestAttrs(r), "agent_id", id, "workspace_id", wsID)...)
	resp := agentToResponse(archived)
	actorType, actorID := h.resolveActor(r, userID, wsID)
	h.publish(protocol.EventAgentArchived, wsID, actorType, actorID, map[string]any{"agent": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) RestoreAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := h.loadAgentForUser(w, r, id)
	if !ok {
		return
	}
	if !h.canManageAgent(w, r, agent) {
		return
	}
	if !agent.ArchivedAt.Valid {
		writeError(w, http.StatusConflict, "agent is not archived")
		return
	}

	restored, err := h.Queries.RestoreAgent(r.Context(), parseUUID(id))
	if err != nil {
		slog.Warn("restore agent failed", append(logger.RequestAttrs(r), "error", err, "agent_id", id)...)
		writeError(w, http.StatusInternalServerError, "failed to restore agent")
		return
	}

	wsID := uuidToString(restored.WorkspaceID)
	slog.Info("agent restored", append(logger.RequestAttrs(r), "agent_id", id, "workspace_id", wsID)...)
	resp := agentToResponse(restored)
	userID := requestUserID(r)
	actorType, actorID := h.resolveActor(r, userID, wsID)
	h.publish(protocol.EventAgentRestored, wsID, actorType, actorID, map[string]any{"agent": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListAgentTasks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := h.loadAgentForUser(w, r, id); !ok {
		return
	}

	tasks, err := h.Queries.ListAgentTasks(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agent tasks")
		return
	}

	resp := make([]AgentTaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = taskToResponse(t)
	}

	writeJSON(w, http.StatusOK, resp)
}
