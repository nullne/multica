package handler

import (
	"context"
	"encoding/json"
	"fmt"
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
	ID                 string  `json:"id"`
	WorkspaceID        string  `json:"workspace_id"`
	Name               string  `json:"name"`
	SourceType         string  `json:"source_type"`
	TokenPrefix        string  `json:"token_prefix"`
	Status             string  `json:"status"`
	DedupWindowSeconds int32   `json:"dedup_window_seconds"`
	BotUserID          *string `json:"bot_user_id"`
	InstallationID     *int64  `json:"installation_id"`
	CreatedBy          string  `json:"created_by"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
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
	resp := WebhookResponse{
		ID:                 uuidToString(w.ID),
		WorkspaceID:        uuidToString(w.WorkspaceID),
		Name:               w.Name,
		SourceType:         w.SourceType,
		TokenPrefix:        w.TokenPrefix,
		Status:             w.Status,
		DedupWindowSeconds: w.DedupWindowSeconds,
		BotUserID:          uuidToPtr(w.BotUserID),
		CreatedBy:          uuidToString(w.CreatedBy),
		CreatedAt:          timestampToString(w.CreatedAt),
		UpdatedAt:          timestampToString(w.UpdatedAt),
	}
	if w.InstallationID.Valid {
		v := w.InstallationID.Int64
		resp.InstallationID = &v
	}
	return resp
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
	Name               string  `json:"name"`
	SourceType         string  `json:"source_type"`
	DedupWindowSeconds *int32  `json:"dedup_window_seconds"`
	BotUserID          *string `json:"bot_user_id"`
	// Inline create_issue action fields. Optional — when absent, the webhook
	// is created with no actions and the caller must add them via the action
	// CRUD endpoints. This is the only path used for source_type='github',
	// which auto-installs a webhook with no default action.
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
	if req.SourceType == "" {
		req.SourceType = "standard"
	}
	if req.SourceType == "github" {
		writeError(w, http.StatusBadRequest, "github webhooks are auto-managed via the GitHub App connect flow")
		return
	}
	if _, err := wh.GetAdapter(req.SourceType); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
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
		BotUserID:          parseOptionalUUID(req.BotUserID),
	})
	if err != nil {
		slog.Warn("create webhook failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create webhook")
		return
	}

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
	BotUserID          *string `json:"bot_user_id"`
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
		if existing.SourceType == "github" && *req.SourceType != "github" {
			writeError(w, http.StatusBadRequest, "github webhook source_type cannot be changed")
			return
		}
		if _, err := wh.GetAdapter(*req.SourceType); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// Allow explicitly clearing the bot user via empty string.
	if req.BotUserID != nil && *req.BotUserID == "" {
		if _, err := h.Queries.ClearWebhookBotUser(r.Context(), parseUUID(id)); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to clear bot user")
			return
		}
		req.BotUserID = nil
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
	if req.BotUserID != nil {
		params.BotUserID = parseUUID(*req.BotUserID)
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

	if existing.SourceType == "github" {
		writeError(w, http.StatusBadRequest, "delete the GitHub App connection instead")
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
	if existing.SourceType == "github" {
		writeError(w, http.StatusBadRequest, "github webhooks are authenticated by HMAC signature, not token")
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

func (h *Handler) ListWorkspaceWebhookEvents(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	limit := int32(100)
	offset := int32(0)
	events, err := h.Queries.ListWorkspaceWebhookEvents(r.Context(), db.ListWorkspaceWebhookEventsParams{
		WorkspaceID: parseUUID(workspaceID),
		Limit:       limit,
		Offset:      offset,
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
	if err := validateActionConfig(existing, req.ActionType, req.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
		merged, err := mergeActionConfig(action, req.Config)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		params.Config = merged
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

// validateActionConfig sanity-checks the incoming action config for required
// fields. comment_issue requires the parent webhook to have a bot user bound
// (because the comment is authored by that bot). create_issue requires
// agent_id.
func validateActionConfig(webhook db.Webhook, actionType string, incoming any) error {
	raw, err := json.Marshal(incoming)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	switch actionType {
	case "create_issue":
		var cfg CreateIssueActionConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("invalid create_issue config: %w", err)
		}
		if cfg.AgentID == "" {
			return fmt.Errorf("agent_id is required in create_issue config")
		}
	case "comment_issue":
		var cfg CommentIssueActionConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("invalid comment_issue config: %w", err)
		}
		if !webhook.BotUserID.Valid {
			return fmt.Errorf("comment_issue action requires the webhook to have a bot user (set bot_user_id)")
		}
	default:
		return fmt.Errorf("unsupported action_type: %s", actionType)
	}
	return nil
}

// mergeActionConfig merges incoming config fields into the existing action
// config. For create_issue actions, this prevents partial updates from
// clobbering required fields like agent_id.
func mergeActionConfig(action db.WebhookAction, incoming any) ([]byte, error) {
	switch action.ActionType {
	case "create_issue":
		var base CreateIssueActionConfig
		if action.Config != nil {
			json.Unmarshal(action.Config, &base)
		}
		incomingJSON, err := json.Marshal(incoming)
		if err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		if err := json.Unmarshal(incomingJSON, &base); err != nil {
			return nil, fmt.Errorf("invalid create_issue config: %w", err)
		}
		if base.AgentID == "" {
			return nil, fmt.Errorf("agent_id is required in create_issue config")
		}
		return json.Marshal(base)
	case "comment_issue":
		var base CommentIssueActionConfig
		if action.Config != nil {
			json.Unmarshal(action.Config, &base)
		}
		incomingJSON, err := json.Marshal(incoming)
		if err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		if err := json.Unmarshal(incomingJSON, &base); err != nil {
			return nil, fmt.Errorf("invalid comment_issue config: %w", err)
		}
		return json.Marshal(base)
	default:
		return json.Marshal(incoming)
	}
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

// --- Action config schemas ---

// CreateIssueActionConfig is the config schema for action_type = "create_issue".
type CreateIssueActionConfig struct {
	AgentID             string   `json:"agent_id"`
	TitleTemplate       string   `json:"title_template"`
	DescriptionTemplate string   `json:"description_template"`
	Labels              []string `json:"labels"`
	DispatchProvider    string   `json:"dispatch_provider,omitempty"`
	DispatchDaemonID    string   `json:"dispatch_daemon_id,omitempty"`
	DispatchDaemonLabel string   `json:"dispatch_daemon_label,omitempty"`
	// Optional event filters; matched against Event.Type and Event.Data["repo"].
	EventTypes []string `json:"event_types,omitempty"`
	Repos      []string `json:"repos,omitempty"`
}

// CommentIssueActionConfig is the config schema for action_type = "comment_issue".
// The action looks up the target Multica issue by Event.Data["source_url"]
// against issue_link, then posts a templated comment as the webhook's bound
// bot user. When MentionAgentID is set, the rendered content is appended with
// an @mention link, which lets the existing on_mention trigger pick it up.
type CommentIssueActionConfig struct {
	ContentTemplate string   `json:"content_template"`
	MentionAgentID  string   `json:"mention_agent_id,omitempty"`
	EventTypes      []string `json:"event_types,omitempty"`
	Repos           []string `json:"repos,omitempty"`
}

// matchesFilters returns true when the action should run for this event,
// based on the EventTypes/Repos filters. Empty filter lists mean "match all".
func actionMatchesFilters(eventTypes, repos []string, evt wh.Event) bool {
	if len(eventTypes) > 0 {
		matched := false
		for _, t := range eventTypes {
			if t == "" {
				continue
			}
			if t == evt.Type || strings.HasPrefix(evt.Type, t+".") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(repos) > 0 {
		repo := evt.Data["repo"]
		matched := false
		for _, r := range repos {
			if r == "" {
				continue
			}
			if r == repo {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// --- Public ingest endpoint (token-authenticated) ---

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

	if webhook.SourceType == "github" {
		writeError(w, http.StatusBadRequest, "send github events to /api/github/events")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	received, created := h.ingestWithWebhook(r.Context(), webhook, body, r.Header)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"received": received,
		"created":  created,
	})
}

// ingestWithWebhook drives the full pipeline for a webhook record once the
// caller has authenticated and resolved the record. It is shared by the
// token-authenticated public endpoint and the GitHub HMAC endpoint.
//
// Returns (events parsed, issues/comments created).
func (h *Handler) ingestWithWebhook(ctx context.Context, webhook db.Webhook, body []byte, headers http.Header) (int, int) {
	if webhook.Status != "active" {
		h.logWebhookEvent(ctx, webhook, "", body, "filtered", pgtype.UUID{}, "webhook is paused")
		return 0, 0
	}

	adapter, err := wh.GetAdapter(webhook.SourceType)
	if err != nil {
		h.logWebhookEvent(ctx, webhook, "", body, "error", pgtype.UUID{}, err.Error())
		return 0, 0
	}

	events, err := adapter.Parse(json.RawMessage(body), headers)
	if err != nil {
		h.logWebhookEvent(ctx, webhook, "", body, "error", pgtype.UUID{}, "parse failed: "+err.Error())
		return 0, 0
	}
	if len(events) == 0 {
		// Adapter chose to drop the event silently (e.g. unsupported sub-action).
		return 0, 0
	}

	actions, err := h.Queries.ListEnabledWebhookActions(ctx, webhook.ID)
	if err != nil {
		h.logWebhookEvent(ctx, webhook, "", body, "error", pgtype.UUID{}, "list actions: "+err.Error())
		return len(events), 0
	}

	created := 0
	for _, evt := range events {
		// Webhook-level dedup. Note this is independent of the per-action filters
		// — once we've processed a dedup key for any action, later events with
		// the same key are skipped entirely.
		if evt.DedupKey != "" && webhook.DedupWindowSeconds > 0 {
			_, dedupErr := h.Queries.FindRecentWebhookEvent(ctx, db.FindRecentWebhookEventParams{
				WebhookID:     webhook.ID,
				DedupKey:      evt.DedupKey,
				WindowSeconds: float64(webhook.DedupWindowSeconds),
			})
			if dedupErr == nil {
				h.logWebhookEvent(ctx, webhook, evt.DedupKey, body, "deduped", pgtype.UUID{}, "")
				continue
			}
		}

		processed := false
		for _, action := range actions {
			ranOK, issueID, runErr := h.runWebhookAction(ctx, webhook, action, evt)
			if runErr != nil {
				h.logWebhookEvent(ctx, webhook, evt.DedupKey, body, "error", pgtype.UUID{}, fmt.Sprintf("%s: %s", action.ActionType, runErr.Error()))
				continue
			}
			if !ranOK {
				continue
			}
			processed = true
			created++
			h.logWebhookEvent(ctx, webhook, evt.DedupKey, body, "processed", issueID, "")
		}
		if !processed {
			h.logWebhookEvent(ctx, webhook, evt.DedupKey, body, "filtered", pgtype.UUID{}, "no matching action")
		}
	}

	return len(events), created
}

// runWebhookAction dispatches a single (event, action) pair. Returns
// (ran, issueID, error). ran=false means the action's filters did not match
// (no error, just skipped). ran=true with a nil error means the action
// produced a side effect; issueID points at the issue if applicable.
func (h *Handler) runWebhookAction(ctx context.Context, webhook db.Webhook, action db.WebhookAction, evt wh.Event) (bool, pgtype.UUID, error) {
	switch action.ActionType {
	case "create_issue":
		var cfg CreateIssueActionConfig
		if err := json.Unmarshal(action.Config, &cfg); err != nil {
			return false, pgtype.UUID{}, fmt.Errorf("invalid action config: %w", err)
		}
		if !actionMatchesFilters(cfg.EventTypes, cfg.Repos, evt) {
			return false, pgtype.UUID{}, nil
		}
		issueID, err := h.executeCreateIssueAction(ctx, webhook, cfg, evt)
		if err != nil {
			return true, pgtype.UUID{}, err
		}
		return true, issueID, nil

	case "comment_issue":
		var cfg CommentIssueActionConfig
		if err := json.Unmarshal(action.Config, &cfg); err != nil {
			return false, pgtype.UUID{}, fmt.Errorf("invalid action config: %w", err)
		}
		if !actionMatchesFilters(cfg.EventTypes, cfg.Repos, evt) {
			return false, pgtype.UUID{}, nil
		}
		issueID, ran, err := h.executeCommentIssueAction(ctx, webhook, cfg, evt)
		if err != nil {
			return true, pgtype.UUID{}, err
		}
		if !ran {
			return false, pgtype.UUID{}, nil
		}
		return true, issueID, nil

	default:
		return false, pgtype.UUID{}, fmt.Errorf("unknown action type: %s", action.ActionType)
	}
}

func (h *Handler) executeCreateIssueAction(ctx context.Context, webhook db.Webhook, cfg CreateIssueActionConfig, evt wh.Event) (pgtype.UUID, error) {
	if cfg.AgentID == "" {
		return pgtype.UUID{}, fmt.Errorf("action config missing agent_id")
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

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return pgtype.UUID{}, err
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)
	issueNumber, err := qtx.IncrementIssueCounter(ctx, webhook.WorkspaceID)
	if err != nil {
		return pgtype.UUID{}, err
	}

	creatorType := "webhook"
	if webhook.SourceType == "github" {
		creatorType = "github"
	}

	issue, err := qtx.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID:         webhook.WorkspaceID,
		Title:               title,
		Description:         strToText(description),
		Status:              "backlog",
		Priority:            priority,
		AssigneeType:        pgtype.Text{String: "agent", Valid: true},
		AssigneeID:          parseUUID(cfg.AgentID),
		CreatorType:         creatorType,
		CreatorID:           webhook.ID,
		Number:              issueNumber,
		DispatchProvider:    strToText(cfg.DispatchProvider),
		DispatchDaemonID:    parseOptionalUUID(&cfg.DispatchDaemonID),
		DispatchDaemonLabel: strToText(cfg.DispatchDaemonLabel),
	})
	if err != nil {
		return pgtype.UUID{}, err
	}

	for _, labelID := range cfg.Labels {
		lid := parseUUID(labelID)
		if lid.Valid {
			_ = qtx.AddLabelToIssue(ctx, db.AddLabelToIssueParams{
				IssueID: issue.ID,
				LabelID: lid,
			})
		}
	}

	// Persist the source link so subsequent events for the same external
	// resource (PR / issue) can find this issue via comment_issue actions.
	if sourceURL := evt.Data["source_url"]; sourceURL != "" {
		kind := evt.Data["source_kind"]
		if kind == "" {
			kind = "issue"
		}
		_, linkErr := qtx.CreateIssueLink(ctx, db.CreateIssueLinkParams{
			IssueID:     issue.ID,
			WorkspaceID: webhook.WorkspaceID,
			SourceType:  webhook.SourceType,
			Kind:        kind,
			Direction:   "source",
			Url:         sourceURL,
			ExternalID:  evt.Data["external_id"],
		})
		if linkErr != nil {
			slog.Warn("create issue_link failed", "issue_id", uuidToString(issue.ID), "url", sourceURL, "error", linkErr)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return pgtype.UUID{}, err
	}

	// Broadcast and trigger agent task. Both happen outside the transaction
	// since they touch external systems (WS hub, task queue) and rely on the
	// committed state.
	workspaceID := uuidToString(webhook.WorkspaceID)
	prefix := h.getIssuePrefix(ctx, issue.WorkspaceID)
	resp := issueToResponse(issue, prefix)
	h.publish(protocol.EventIssueCreated, workspaceID, "system", "", map[string]any{"issue": resp})

	if h.shouldEnqueueAgentTask(ctx, issue) {
		if _, err := h.TaskService.EnqueueTaskForIssue(ctx, issue); err != nil {
			slog.Warn("webhook: enqueue agent task failed", "issue_id", uuidToString(issue.ID), "error", err)
		}
	}

	return issue.ID, nil
}

// executeCommentIssueAction looks up the target Multica issue via issue_link
// (using Event.Data["source_url"]) and posts a templated comment authored by
// the webhook's bot user. When the comment mentions an agent, the existing
// on_mention pipeline (enqueueMentionedAgentTasks) will trigger that agent.
//
// Returns (issueID, ran, error). ran=false means the source URL had no
// matching issue; the action is silently skipped (logged as 'filtered' by the
// caller).
func (h *Handler) executeCommentIssueAction(ctx context.Context, webhook db.Webhook, cfg CommentIssueActionConfig, evt wh.Event) (pgtype.UUID, bool, error) {
	if !webhook.BotUserID.Valid {
		return pgtype.UUID{}, false, fmt.Errorf("webhook has no bot_user_id")
	}

	sourceURL := evt.Data["source_url"]
	if sourceURL == "" {
		return pgtype.UUID{}, false, nil
	}

	link, err := h.Queries.GetIssueLinkByURL(ctx, db.GetIssueLinkByURLParams{
		WorkspaceID: webhook.WorkspaceID,
		Url:         sourceURL,
	})
	if err != nil {
		// Not finding a matching link is normal — a Multica issue was never
		// created for this PR. Treat as "no action".
		if isNotFound(err) {
			return pgtype.UUID{}, false, nil
		}
		return pgtype.UUID{}, false, fmt.Errorf("lookup issue_link: %w", err)
	}

	issue, err := h.Queries.GetIssue(ctx, link.IssueID)
	if err != nil {
		return pgtype.UUID{}, false, fmt.Errorf("load issue: %w", err)
	}

	content := renderTemplate(cfg.ContentTemplate, evt.Data)
	if content == "" {
		content = evt.Data["body"]
	}
	if content == "" {
		content = "(empty webhook event)"
	}

	if cfg.MentionAgentID != "" {
		agent, agentErr := h.Queries.GetAgent(ctx, parseUUID(cfg.MentionAgentID))
		if agentErr == nil && uuidToString(agent.WorkspaceID) == uuidToString(webhook.WorkspaceID) {
			content = strings.TrimRight(content, "\n") + fmt.Sprintf("\n\n[@%s](mention://agent/%s)", agent.Name, cfg.MentionAgentID)
		} else {
			slog.Warn("comment_issue: mention agent not found in workspace",
				"agent_id", cfg.MentionAgentID,
				"webhook_id", uuidToString(webhook.ID),
			)
		}
	}

	comment, err := h.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "member",
		AuthorID:    webhook.BotUserID,
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
	botID := uuidToString(webhook.BotUserID)
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "member", botID, map[string]any{
		"comment":             commentForBroadcast(comment),
		"issue_title":         issue.Title,
		"issue_assignee_type": textToPtr(issue.AssigneeType),
		"issue_assignee_id":   uuidToPtr(issue.AssigneeID),
		"issue_status":        issue.Status,
	})

	if h.shouldEnqueueOnComment(ctx, issue) && !h.commentMentionsOthersButNotAssignee(comment.Content, issue) {
		replyTo := comment.ID
		if comment.ParentID.Valid {
			replyTo = comment.ParentID
		}
		if _, err := h.TaskService.EnqueueTaskForIssue(ctx, issue, replyTo); err != nil {
			slog.Warn("webhook comment: enqueue assignee task failed", "issue_id", uuidToString(issue.ID), "error", err)
		}
	}

	h.enqueueMentionedAgentTasks(ctx, issue, comment, "member", botID)

	return issue.ID, true, nil
}

// commentForBroadcast builds the lightweight payload published over WS for
// new comments. We avoid pulling reactions/attachments here since webhook
// comments never have either at creation time.
func commentForBroadcast(c db.Comment) map[string]any {
	return map[string]any{
		"id":           uuidToString(c.ID),
		"issue_id":     uuidToString(c.IssueID),
		"workspace_id": uuidToString(c.WorkspaceID),
		"author_type":  c.AuthorType,
		"author_id":    uuidToString(c.AuthorID),
		"content":      c.Content,
		"type":         c.Type,
		"parent_id":    uuidToPtr(c.ParentID),
		"created_at":   timestampToString(c.CreatedAt),
		"updated_at":   timestampToString(c.UpdatedAt),
	}
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

func (h *Handler) logWebhookEvent(ctx context.Context, webhook db.Webhook, dedupKey string, payload []byte, status string, issueID pgtype.UUID, errMsg string) {
	var errorMessage pgtype.Text
	if errMsg != "" {
		errorMessage = pgtype.Text{String: errMsg, Valid: true}
	}
	_, err := h.Queries.CreateWebhookEventLog(ctx, db.CreateWebhookEventLogParams{
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

