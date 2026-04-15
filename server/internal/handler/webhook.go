package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/auth"
	"github.com/nullne/multica/server/internal/logger"
	wh "github.com/nullne/multica/server/internal/webhook"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

// --- Response types ---

type WebhookResponse struct {
	ID                 string `json:"id"`
	WorkspaceID        string `json:"workspace_id"`
	Name               string `json:"name"`
	SourceType         string `json:"source_type"`
	TokenPrefix        string `json:"token_prefix"`
	Status             string `json:"status"`
	DedupWindowSeconds int32  `json:"dedup_window_seconds"`
	CreatedBy          string `json:"created_by"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

type CreateWebhookTokenResponse struct {
	WebhookResponse
	Token string `json:"token"`
}

type WebhookActionResponse struct {
	ID         string `json:"id"`
	WebhookID  string `json:"webhook_id"`
	ActionType string `json:"action_type"`
	Config     any    `json:"config"`
	Enabled    bool   `json:"enabled"`
	Position   int32  `json:"position"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type WebhookEventResponse struct {
	ID           string  `json:"id"`
	WebhookID    string  `json:"webhook_id"`
	DedupKey     string  `json:"dedup_key"`
	Payload      any     `json:"payload"`
	Status       string  `json:"status"`
	IssueID      *string `json:"issue_id"`
	ErrorMessage *string `json:"error_message"`
	CreatedAt    string  `json:"created_at"`
}

func webhookToResponse(w db.Webhook) WebhookResponse {
	return WebhookResponse{
		ID:                 uuidToString(w.ID),
		WorkspaceID:        uuidToString(w.WorkspaceID),
		Name:               w.Name,
		SourceType:         w.SourceType,
		TokenPrefix:        w.TokenPrefix,
		Status:             w.Status,
		DedupWindowSeconds: w.DedupWindowSeconds,
		CreatedBy:          uuidToString(w.CreatedBy),
		CreatedAt:          timestampToString(w.CreatedAt),
		UpdatedAt:          timestampToString(w.UpdatedAt),
	}
}

func webhookActionToResponse(a db.WebhookAction) WebhookActionResponse {
	var config any
	if a.Config != nil {
		json.Unmarshal(a.Config, &config)
	}
	return WebhookActionResponse{
		ID:         uuidToString(a.ID),
		WebhookID:  uuidToString(a.WebhookID),
		ActionType: a.ActionType,
		Config:     config,
		Enabled:    a.Enabled,
		Position:   a.Position,
		CreatedAt:  timestampToString(a.CreatedAt),
		UpdatedAt:  timestampToString(a.UpdatedAt),
	}
}

func webhookEventToResponse(e db.WebhookEventLog) WebhookEventResponse {
	var payload any
	if e.Payload != nil {
		json.Unmarshal(e.Payload, &payload)
	}
	return WebhookEventResponse{
		ID:           uuidToString(e.ID),
		WebhookID:    uuidToString(e.WebhookID),
		DedupKey:     e.DedupKey,
		Payload:      payload,
		Status:       e.Status,
		IssueID:      uuidToPtr(e.IssueID),
		ErrorMessage: textToPtr(e.ErrorMessage),
		CreatedAt:    timestampToString(e.CreatedAt),
	}
}

// --- Webhook CRUD ---

type CreateWebhookRequest struct {
	Name               string `json:"name"`
	SourceType         string `json:"source_type"`
	DedupWindowSeconds *int32 `json:"dedup_window_seconds"`
	// P0: action fields sent inline, creates a default create_issue action.
	AgentID             string   `json:"agent_id"`
	TitleTemplate       string   `json:"title_template"`
	DescriptionTemplate string   `json:"description_template"`
	Labels              []string `json:"labels"`
	DispatchProvider    string   `json:"dispatch_provider"`
	DispatchDaemonID    string   `json:"dispatch_daemon_id"`
	DispatchDaemonLabel string   `json:"dispatch_daemon_label"`
}

func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := resolveWorkspaceID(r)

	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	if req.SourceType == "" {
		req.SourceType = "standard"
	}
	if _, err := wh.GetAdapter(req.SourceType); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dedupWindow := int32(600)
	if req.DedupWindowSeconds != nil {
		dedupWindow = *req.DedupWindowSeconds
	}

	rawToken, err := wh.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	prefix := rawToken
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	webhook, err := qtx.CreateWebhook(r.Context(), db.CreateWebhookParams{
		WorkspaceID:        parseUUID(workspaceID),
		Name:               req.Name,
		SourceType:         req.SourceType,
		TokenHash:          auth.HashToken(rawToken),
		TokenPrefix:        prefix,
		DedupWindowSeconds: dedupWindow,
		CreatedBy:          parseUUID(userID),
	})
	if err != nil {
		slog.Warn("create webhook failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create webhook")
		return
	}

	// Build action config from the inline fields.
	if req.Labels == nil {
		req.Labels = []string{}
	}
	actionConfig := CreateIssueActionConfig{
		AgentID:             req.AgentID,
		TitleTemplate:       req.TitleTemplate,
		DescriptionTemplate: req.DescriptionTemplate,
		Labels:              req.Labels,
		DispatchProvider:    req.DispatchProvider,
		DispatchDaemonID:    req.DispatchDaemonID,
		DispatchDaemonLabel: req.DispatchDaemonLabel,
	}
	configJSON, err := json.Marshal(actionConfig)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal action config")
		return
	}

	action, err := qtx.CreateWebhookAction(r.Context(), db.CreateWebhookActionParams{
		WebhookID:  webhook.ID,
		ActionType: "create_issue",
		Config:     configJSON,
		Enabled:    true,
		Position:   0,
	})
	if err != nil {
		slog.Warn("create webhook action failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create webhook action")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	slog.Info("webhook created", append(logger.RequestAttrs(r), "webhook_id", uuidToString(webhook.ID), "name", webhook.Name)...)
	writeJSON(w, http.StatusCreated, map[string]any{
		"webhook": webhookToResponse(webhook),
		"token":   rawToken,
		"actions": []WebhookActionResponse{webhookActionToResponse(action)},
	})
}

func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	webhooks, err := h.Queries.ListWebhooksByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}

	resp := make([]map[string]any, len(webhooks))
	for i, wk := range webhooks {
		count, _ := h.Queries.CountWebhookEvents(r.Context(), wk.ID)
		actions, _ := h.Queries.ListWebhookActions(r.Context(), wk.ID)
		actionResponses := make([]WebhookActionResponse, len(actions))
		for j, a := range actions {
			actionResponses[j] = webhookActionToResponse(a)
		}
		resp[i] = map[string]any{
			"webhook":     webhookToResponse(wk),
			"event_count": count,
			"actions":     actionResponses,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)

	webhook, err := h.Queries.GetWebhook(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	if uuidToString(webhook.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	count, _ := h.Queries.CountWebhookEvents(r.Context(), webhook.ID)
	actions, _ := h.Queries.ListWebhookActions(r.Context(), webhook.ID)
	actionResponses := make([]WebhookActionResponse, len(actions))
	for i, a := range actions {
		actionResponses[i] = webhookActionToResponse(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"webhook":     webhookToResponse(webhook),
		"event_count": count,
		"actions":     actionResponses,
	})
}

type UpdateWebhookRequest struct {
	Name               *string `json:"name"`
	SourceType         *string `json:"source_type"`
	Status             *string `json:"status"`
	DedupWindowSeconds *int32  `json:"dedup_window_seconds"`
}

func (h *Handler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)

	existing, err := h.Queries.GetWebhook(r.Context(), parseUUID(id))
	if err != nil || uuidToString(existing.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	var req UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SourceType != nil {
		if _, err := wh.GetAdapter(*req.SourceType); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	params := db.UpdateWebhookParams{ID: parseUUID(id)}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.SourceType != nil {
		params.SourceType = pgtype.Text{String: *req.SourceType, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.DedupWindowSeconds != nil {
		params.DedupWindowSeconds = pgtype.Int4{Int32: *req.DedupWindowSeconds, Valid: true}
	}

	webhook, err := h.Queries.UpdateWebhook(r.Context(), params)
	if err != nil {
		slog.Warn("update webhook failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to update webhook")
		return
	}

	writeJSON(w, http.StatusOK, webhookToResponse(webhook))
}

func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)

	existing, err := h.Queries.GetWebhook(r.Context(), parseUUID(id))
	if err != nil || uuidToString(existing.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	if err := h.Queries.DeleteWebhook(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RegenerateWebhookToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)

	existing, err := h.Queries.GetWebhook(r.Context(), parseUUID(id))
	if err != nil || uuidToString(existing.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	rawToken, err := wh.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	prefix := rawToken
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}

	webhook, err := h.Queries.RegenerateWebhookToken(r.Context(), db.RegenerateWebhookTokenParams{
		ID:          parseUUID(id),
		TokenHash:   auth.HashToken(rawToken),
		TokenPrefix: prefix,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to regenerate token")
		return
	}

	writeJSON(w, http.StatusOK, CreateWebhookTokenResponse{
		WebhookResponse: webhookToResponse(webhook),
		Token:           rawToken,
	})
}

func (h *Handler) ListWebhookEvents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)

	existing, err := h.Queries.GetWebhook(r.Context(), parseUUID(id))
	if err != nil || uuidToString(existing.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	limit := int32(50)
	offset := int32(0)
	events, err := h.Queries.ListWebhookEvents(r.Context(), db.ListWebhookEventsParams{
		WebhookID: parseUUID(id),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}

	resp := make([]WebhookEventResponse, len(events))
	for i, e := range events {
		resp[i] = webhookEventToResponse(e)
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- Webhook Action CRUD ---

type CreateWebhookActionRequest struct {
	ActionType string `json:"action_type"`
	Config     any    `json:"config"`
	Enabled    *bool  `json:"enabled"`
	Position   *int32 `json:"position"`
}

func (h *Handler) CreateWebhookAction(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)

	existing, err := h.Queries.GetWebhook(r.Context(), parseUUID(webhookID))
	if err != nil || uuidToString(existing.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	var req CreateWebhookActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ActionType == "" {
		req.ActionType = "create_issue"
	}

	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid config")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	position := int32(0)
	if req.Position != nil {
		position = *req.Position
	}

	action, err := h.Queries.CreateWebhookAction(r.Context(), db.CreateWebhookActionParams{
		WebhookID:  parseUUID(webhookID),
		ActionType: req.ActionType,
		Config:     configJSON,
		Enabled:    enabled,
		Position:   position,
	})
	if err != nil {
		slog.Warn("create webhook action failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create action")
		return
	}

	writeJSON(w, http.StatusCreated, webhookActionToResponse(action))
}

func (h *Handler) ListWebhookActions(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)

	existing, err := h.Queries.GetWebhook(r.Context(), parseUUID(webhookID))
	if err != nil || uuidToString(existing.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	actions, err := h.Queries.ListWebhookActions(r.Context(), parseUUID(webhookID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list actions")
		return
	}

	resp := make([]WebhookActionResponse, len(actions))
	for i, a := range actions {
		resp[i] = webhookActionToResponse(a)
	}
	writeJSON(w, http.StatusOK, resp)
}

type UpdateWebhookActionRequest struct {
	ActionType *string `json:"action_type"`
	Config     any     `json:"config"`
	Enabled    *bool   `json:"enabled"`
	Position   *int32  `json:"position"`
}

func (h *Handler) UpdateWebhookAction(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	actionID := chi.URLParam(r, "actionId")
	workspaceID := resolveWorkspaceID(r)

	existing, err := h.Queries.GetWebhook(r.Context(), parseUUID(webhookID))
	if err != nil || uuidToString(existing.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	action, err := h.Queries.GetWebhookAction(r.Context(), parseUUID(actionID))
	if err != nil || uuidToString(action.WebhookID) != webhookID {
		writeError(w, http.StatusNotFound, "action not found")
		return
	}

	var req UpdateWebhookActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateWebhookActionParams{ID: parseUUID(actionID)}
	if req.ActionType != nil {
		params.ActionType = pgtype.Text{String: *req.ActionType, Valid: true}
	}
	if req.Config != nil {
		configJSON, err := json.Marshal(req.Config)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid config")
			return
		}
		params.Config = configJSON
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	}
	if req.Position != nil {
		params.Position = pgtype.Int4{Int32: *req.Position, Valid: true}
	}

	updated, err := h.Queries.UpdateWebhookAction(r.Context(), params)
	if err != nil {
		slog.Warn("update webhook action failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to update action")
		return
	}

	writeJSON(w, http.StatusOK, webhookActionToResponse(updated))
}

func (h *Handler) DeleteWebhookAction(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	actionID := chi.URLParam(r, "actionId")
	workspaceID := resolveWorkspaceID(r)

	existing, err := h.Queries.GetWebhook(r.Context(), parseUUID(webhookID))
	if err != nil || uuidToString(existing.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	action, err := h.Queries.GetWebhookAction(r.Context(), parseUUID(actionID))
	if err != nil || uuidToString(action.WebhookID) != webhookID {
		writeError(w, http.StatusNotFound, "action not found")
		return
	}

	if err := h.Queries.DeleteWebhookAction(r.Context(), parseUUID(actionID)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete action")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Adapter Info ---

func (h *Handler) ListWebhookAdapters(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, wh.ListAdapters())
}

// --- Public Ingest Endpoint (no JWT required, token-authenticated) ---

// CreateIssueActionConfig is the config schema for action_type = "create_issue".
type CreateIssueActionConfig struct {
	AgentID             string   `json:"agent_id"`
	TitleTemplate       string   `json:"title_template"`
	DescriptionTemplate string   `json:"description_template"`
	Labels              []string `json:"labels"`
	DispatchProvider    string   `json:"dispatch_provider,omitempty"`
	DispatchDaemonID    string   `json:"dispatch_daemon_id,omitempty"`
	DispatchDaemonLabel string   `json:"dispatch_daemon_label,omitempty"`
}

func (h *Handler) IngestWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	authHeader := r.Header.Get("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == "" || tokenString == authHeader {
		writeError(w, http.StatusUnauthorized, "missing or invalid authorization")
		return
	}

	webhook, err := h.Queries.GetWebhookByTokenHash(r.Context(), auth.HashToken(tokenString))
	if err != nil || uuidToString(webhook.ID) != id {
		writeError(w, http.StatusUnauthorized, "invalid webhook token")
		return
	}

	if webhook.Status != "active" {
		writeError(w, http.StatusConflict, "webhook is paused")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	adapter, err := wh.GetAdapter(webhook.SourceType)
	if err != nil {
		h.logWebhookEvent(r, webhook, "", body, "error", pgtype.UUID{}, err.Error())
		writeError(w, http.StatusInternalServerError, "unsupported source type")
		return
	}

	events, err := adapter.Parse(json.RawMessage(body), r.Header)
	if err != nil {
		h.logWebhookEvent(r, webhook, "", body, "error", pgtype.UUID{}, "parse failed: "+err.Error())
		writeError(w, http.StatusBadRequest, "failed to parse payload: "+err.Error())
		return
	}

	actions, err := h.Queries.ListEnabledWebhookActions(r.Context(), webhook.ID)
	if err != nil {
		h.logWebhookEvent(r, webhook, "", body, "error", pgtype.UUID{}, "list actions: "+err.Error())
		writeError(w, http.StatusInternalServerError, "failed to load actions")
		return
	}

	workspaceID := uuidToString(webhook.WorkspaceID)
	created := 0

	for _, evt := range events {
		// Dedup check.
		if evt.DedupKey != "" && webhook.DedupWindowSeconds > 0 {
			_, dedupErr := h.Queries.FindRecentWebhookEvent(r.Context(), db.FindRecentWebhookEventParams{
				WebhookID:     webhook.ID,
				DedupKey:      evt.DedupKey,
				WindowSeconds: float64(webhook.DedupWindowSeconds),
			})
			if dedupErr == nil {
				h.logWebhookEvent(r, webhook, evt.DedupKey, body, "deduped", pgtype.UUID{}, "")
				continue
			}
		}

		for _, action := range actions {
			switch action.ActionType {
			case "create_issue":
				issueID, err := h.executeCreateIssueAction(r, webhook, action, evt)
				if err != nil {
					h.logWebhookEvent(r, webhook, evt.DedupKey, body, "error", pgtype.UUID{}, "create_issue: "+err.Error())
					continue
				}
				h.logWebhookEvent(r, webhook, evt.DedupKey, body, "processed", issueID, "")

				// Broadcast and trigger agent task.
				issue, err := h.Queries.GetIssue(r.Context(), issueID)
				if err == nil {
					prefix := h.getIssuePrefix(r.Context(), issue.WorkspaceID)
					resp := issueToResponse(issue, prefix)
					h.publish(protocol.EventIssueCreated, workspaceID, "system", "", map[string]any{"issue": resp})

					if h.shouldEnqueueAgentTask(r.Context(), issue) {
						if _, err := h.TaskService.EnqueueTaskForIssue(r.Context(), issue); err != nil {
							slog.Warn("webhook: enqueue agent task failed", "issue_id", uuidToString(issue.ID), "error", err)
						}
					}
				}

				created++
			default:
				slog.Warn("webhook: unknown action type", "action_type", action.ActionType, "action_id", uuidToString(action.ID))
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"received": len(events),
		"created":  created,
	})
}

func (h *Handler) executeCreateIssueAction(r *http.Request, webhook db.Webhook, action db.WebhookAction, evt wh.Event) (pgtype.UUID, error) {
	var cfg CreateIssueActionConfig
	if err := json.Unmarshal(action.Config, &cfg); err != nil {
		return pgtype.UUID{}, err
	}

	title := renderTemplate(cfg.TitleTemplate, evt.Data)
	if title == "" {
		title = evt.Data["title"]
	}
	if title == "" {
		title = "Webhook event"
	}

	description := renderTemplate(cfg.DescriptionTemplate, evt.Data)
	if description == "" {
		description = evt.Data["body"]
	}

	priority := evt.Data["priority"]
	if priority == "" {
		priority = "medium"
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		return pgtype.UUID{}, err
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)
	issueNumber, err := qtx.IncrementIssueCounter(r.Context(), webhook.WorkspaceID)
	if err != nil {
		return pgtype.UUID{}, err
	}

	issue, err := qtx.CreateIssue(r.Context(), db.CreateIssueParams{
		WorkspaceID:         webhook.WorkspaceID,
		Title:               title,
		Description:         strToText(description),
		Status:              "backlog",
		Priority:            priority,
		AssigneeType:        pgtype.Text{String: "agent", Valid: true},
		AssigneeID:          parseUUID(cfg.AgentID),
		CreatorType:         "webhook",
		CreatorID:           webhook.ID,
		Number:              issueNumber,
		DispatchProvider:    strToText(cfg.DispatchProvider),
		DispatchDaemonID:    parseOptionalUUID(&cfg.DispatchDaemonID),
		DispatchDaemonLabel: ptrToText(&cfg.DispatchDaemonLabel),
	})
	if err != nil {
		return pgtype.UUID{}, err
	}

	if err := tx.Commit(r.Context()); err != nil {
		return pgtype.UUID{}, err
	}

	return issue.ID, nil
}

// renderTemplate replaces {{.key}} placeholders with values from data.
func renderTemplate(tmpl string, data map[string]string) string {
	if tmpl == "" {
		return ""
	}
	result := tmpl
	for k, v := range data {
		result = strings.ReplaceAll(result, "{{."+k+"}}", v)
	}
	return result
}

func (h *Handler) logWebhookEvent(r *http.Request, webhook db.Webhook, dedupKey string, payload []byte, status string, issueID pgtype.UUID, errMsg string) {
	var errorMessage pgtype.Text
	if errMsg != "" {
		errorMessage = pgtype.Text{String: errMsg, Valid: true}
	}
	_, err := h.Queries.CreateWebhookEventLog(r.Context(), db.CreateWebhookEventLogParams{
		WebhookID:    webhook.ID,
		DedupKey:     dedupKey,
		Payload:      payload,
		Status:       status,
		IssueID:      issueID,
		ErrorMessage: errorMessage,
	})
	if err != nil {
		slog.Warn("failed to log webhook event", "webhook_id", uuidToString(webhook.ID), "error", err)
	}
}
