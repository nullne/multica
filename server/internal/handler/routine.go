package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/auth"
	"github.com/nullne/multica/server/internal/cron"
	"github.com/nullne/multica/server/internal/service"
	wh "github.com/nullne/multica/server/internal/webhook"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

var errRoutineTriggerTokenRequired = errors.New("routine trigger token must be generated before saving")

const routineTriggerMaxPayloadBytes = 1 << 20

type RoutineResponse struct {
	ID                   string                   `json:"id"`
	WorkspaceID          string                   `json:"workspace_id"`
	Name                 string                   `json:"name"`
	Instructions         *string                  `json:"instructions"`
	Priority             string                   `json:"priority"`
	AssigneeType         *string                  `json:"assignee_type"`
	AssigneeID           *string                  `json:"assignee_id"`
	DueDateOffsetHours   *int32                   `json:"due_date_offset_hours"`
	DispatchProvider     *string                  `json:"dispatch_provider"`
	DispatchDaemonID     *string                  `json:"dispatch_daemon_id"`
	DispatchDaemonLabel  *string                  `json:"dispatch_daemon_label"`
	Enabled              bool                     `json:"enabled"`
	GithubAutoFixEnabled bool                     `json:"github_auto_fix_enabled"`
	Managed              bool                     `json:"managed"`
	CreatedByID          string                   `json:"created_by_id"`
	CreatedByType        string                   `json:"created_by_type"`
	CreatedAt            string                   `json:"created_at"`
	UpdatedAt            string                   `json:"updated_at"`
	SubscriberIDs        []string                 `json:"subscriber_ids"`
	LabelIDs             []string                 `json:"label_ids"`
	Triggers             []RoutineTriggerResponse `json:"triggers"`
	Actions              []RoutineActionResponse  `json:"actions"`
}

type RoutineTriggerResponse struct {
	ID                  string  `json:"id"`
	RoutineID           string  `json:"routine_id"`
	TriggerType         string  `json:"trigger_type"`
	SourceType          *string `json:"source_type"`
	TokenPrefix         string  `json:"token_prefix"`
	InstallationID      *int64  `json:"installation_id"`
	Schedule            *string `json:"schedule"`
	Timezone            string  `json:"timezone"`
	RunAt               *string `json:"run_at"`
	NextRunAt           *string `json:"next_run_at"`
	LastTriggeredAt     *string `json:"last_triggered_at"`
	DedupWindowSeconds  int32   `json:"dedup_window_seconds"`
	MaxRuns             *int32  `json:"max_runs"`
	SuccessfulRunsCount int32   `json:"successful_runs_count"`
	Config              any     `json:"config"`
	Enabled             bool    `json:"enabled"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	Token               string  `json:"token,omitempty"`
}

type RoutineActionResponse struct {
	ID         string `json:"id"`
	RoutineID  string `json:"routine_id"`
	ActionType string `json:"action_type"`
	Config     any    `json:"config"`
	Enabled    bool   `json:"enabled"`
	Position   int32  `json:"position"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type RoutineRunResponse struct {
	ID           string         `json:"id"`
	RoutineID    string         `json:"routine_id"`
	TriggerID    *string        `json:"trigger_id"`
	ActionID     *string        `json:"action_id"`
	EventType    string         `json:"event_type"`
	DedupKey     string         `json:"dedup_key"`
	Payload      any            `json:"payload"`
	Status       string         `json:"status"`
	IssueID      *string        `json:"issue_id"`
	CommentID    *string        `json:"comment_id"`
	ErrorMessage *string        `json:"error_message"`
	CreatedAt    string         `json:"created_at"`
	Issue        *IssueResponse `json:"issue,omitempty"`
}

type RoutineTriggerTokenResponse struct {
	Trigger RoutineTriggerResponse `json:"trigger"`
	Token   string                 `json:"token"`
}

type RoutineTriggerTokenDraftResponse struct {
	DraftID     string `json:"draft_id"`
	TokenPrefix string `json:"token_prefix"`
	Token       string `json:"token"`
}

type CreateRoutineRequest struct {
	Name                 string                  `json:"name"`
	Instructions         *string                 `json:"instructions"`
	Priority             *string                 `json:"priority"`
	AssigneeType         *string                 `json:"assignee_type"`
	AssigneeID           *string                 `json:"assignee_id"`
	DueDateOffsetHours   *int32                  `json:"due_date_offset_hours"`
	DispatchProvider     *string                 `json:"dispatch_provider"`
	DispatchDaemonID     *string                 `json:"dispatch_daemon_id"`
	DispatchDaemonLabel  *string                 `json:"dispatch_daemon_label"`
	Enabled              *bool                   `json:"enabled"`
	GithubAutoFixEnabled *bool                   `json:"github_auto_fix_enabled"`
	SubscriberIDs        []string                `json:"subscriber_ids"`
	LabelIDs             []string                `json:"label_ids"`
	Triggers             []RoutineTriggerRequest `json:"triggers"`
	Actions              []RoutineActionRequest  `json:"actions"`
}

type UpdateRoutineRequest = CreateRoutineRequest

type RoutineTriggerRequest struct {
	ID                 *string `json:"id"`
	TriggerType        string  `json:"trigger_type"`
	SourceType         *string `json:"source_type"`
	TokenDraftID       *string `json:"token_draft_id"`
	Schedule           *string `json:"schedule"`
	Timezone           *string `json:"timezone"`
	RunAt              *string `json:"run_at"`
	DedupWindowSeconds *int32  `json:"dedup_window_seconds"`
	MaxRuns            *int32  `json:"max_runs"`
	Config             any     `json:"config"`
	Enabled            *bool   `json:"enabled"`
}

type RoutineActionRequest struct {
	ActionType string `json:"action_type"`
	Config     any    `json:"config"`
	Enabled    *bool  `json:"enabled"`
	Position   *int32 `json:"position"`
}

func routineToResponse(r db.Routine, subscriberIDs, labelIDs []string, triggers []RoutineTriggerResponse, actions []RoutineActionResponse) RoutineResponse {
	var dueDateOffset *int32
	if r.DueDateOffsetHours.Valid {
		dueDateOffset = &r.DueDateOffsetHours.Int32
	}
	return RoutineResponse{
		ID:                   uuidToString(r.ID),
		WorkspaceID:          uuidToString(r.WorkspaceID),
		Name:                 r.Name,
		Instructions:         textToPtr(r.Instructions),
		Priority:             r.Priority,
		AssigneeType:         textToPtr(r.AssigneeType),
		AssigneeID:           uuidToPtr(r.AssigneeID),
		DueDateOffsetHours:   dueDateOffset,
		DispatchProvider:     textToPtr(r.DispatchProvider),
		DispatchDaemonID:     uuidToPtr(r.DispatchDaemonID),
		DispatchDaemonLabel:  textToPtr(r.DispatchDaemonLabel),
		Enabled:              r.Enabled,
		GithubAutoFixEnabled: r.GithubAutoFixEnabled,
		Managed:              r.Managed,
		CreatedByID:          uuidToString(r.CreatedByID),
		CreatedByType:        r.CreatedByType,
		CreatedAt:            timestampToString(r.CreatedAt),
		UpdatedAt:            timestampToString(r.UpdatedAt),
		SubscriberIDs:        subscriberIDs,
		LabelIDs:             labelIDs,
		Triggers:             triggers,
		Actions:              actions,
	}
}

func routineTriggerToResponse(t db.RoutineTrigger, token string) RoutineTriggerResponse {
	var config any
	if len(t.Config) > 0 {
		_ = json.Unmarshal(t.Config, &config)
	}
	var installationID *int64
	if t.InstallationID.Valid {
		installationID = &t.InstallationID.Int64
	}
	var maxRuns *int32
	if t.MaxRuns.Valid {
		maxRuns = &t.MaxRuns.Int32
	}
	return RoutineTriggerResponse{
		ID:                  uuidToString(t.ID),
		RoutineID:           uuidToString(t.RoutineID),
		TriggerType:         t.TriggerType,
		SourceType:          textToPtr(t.SourceType),
		TokenPrefix:         t.TokenPrefix,
		InstallationID:      installationID,
		Schedule:            textToPtr(t.Schedule),
		Timezone:            t.Timezone,
		RunAt:               timestampToPtr(t.RunAt),
		NextRunAt:           timestampToPtr(t.NextRunAt),
		LastTriggeredAt:     timestampToPtr(t.LastTriggeredAt),
		DedupWindowSeconds:  t.DedupWindowSeconds,
		MaxRuns:             maxRuns,
		SuccessfulRunsCount: t.SuccessfulRunsCount,
		Config:              config,
		Enabled:             t.Enabled,
		CreatedAt:           timestampToString(t.CreatedAt),
		UpdatedAt:           timestampToString(t.UpdatedAt),
		Token:               token,
	}
}

func routineActionToResponse(a db.RoutineAction) RoutineActionResponse {
	var config any
	if len(a.Config) > 0 {
		_ = json.Unmarshal(a.Config, &config)
	}
	return RoutineActionResponse{
		ID:         uuidToString(a.ID),
		RoutineID:  uuidToString(a.RoutineID),
		ActionType: a.ActionType,
		Config:     config,
		Enabled:    a.Enabled,
		Position:   a.Position,
		CreatedAt:  timestampToString(a.CreatedAt),
		UpdatedAt:  timestampToString(a.UpdatedAt),
	}
}

func (h *Handler) ListRoutines(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	routines, err := h.Queries.ListRoutines(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list routines")
		return
	}
	resp := make([]RoutineResponse, 0, len(routines))
	for _, routine := range routines {
		full, err := h.buildRoutineResponse(r.Context(), routine, nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load routine")
			return
		}
		resp = append(resp, full)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetRoutine(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	routine, err := h.Queries.GetRoutineInWorkspace(r.Context(), db.GetRoutineInWorkspaceParams{
		ID:          parseUUID(chi.URLParam(r, "id")),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "routine not found")
		return
	}
	resp, err := h.buildRoutineResponse(r.Context(), routine, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load routine")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateRoutine(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	var req CreateRoutineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if msg := validateRoutineRequest(req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	routine, triggerTokens, err := h.saveRoutine(r.Context(), parseUUID(workspaceID), member.UserID, "member", req, nil)
	if err != nil {
		if errors.Is(err, errRoutineTriggerTokenRequired) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		slog.Warn("create routine failed", "workspace_id", workspaceID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create routine")
		return
	}
	resp, err := h.buildRoutineResponse(r.Context(), routine, triggerTokens)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load routine")
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateRoutine(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	id := parseUUID(chi.URLParam(r, "id"))
	existing, err := h.Queries.GetRoutineInWorkspace(r.Context(), db.GetRoutineInWorkspaceParams{
		ID:          id,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "routine not found")
		return
	}
	if existing.Managed {
		writeError(w, http.StatusForbidden, "managed routines cannot be edited")
		return
	}
	var req UpdateRoutineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if msg := validateRoutineRequest(req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	routine, triggerTokens, err := h.saveRoutine(r.Context(), parseUUID(workspaceID), existing.CreatedByID, existing.CreatedByType, req, &existing.ID)
	if err != nil {
		if errors.Is(err, errRoutineTriggerTokenRequired) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		slog.Warn("update routine failed", "routine_id", uuidToString(id), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update routine")
		return
	}
	resp, err := h.buildRoutineResponse(r.Context(), routine, triggerTokens)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load routine")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteRoutine(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	id := parseUUID(chi.URLParam(r, "id"))
	existing, err := h.Queries.GetRoutineInWorkspace(r.Context(), db.GetRoutineInWorkspaceParams{
		ID:          id,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "routine not found")
		return
	}
	if existing.Managed {
		writeError(w, http.StatusForbidden, "managed routines cannot be deleted")
		return
	}
	if err := h.Queries.DeleteRoutine(r.Context(), db.DeleteRoutineParams{
		ID:          id,
		WorkspaceID: parseUUID(workspaceID),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete routine")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TriggerRoutine(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	id := parseUUID(chi.URLParam(r, "id"))
	routine, err := h.Queries.GetRoutineInWorkspace(r.Context(), db.GetRoutineInWorkspaceParams{
		ID:          id,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "routine not found")
		return
	}
	if routine.Managed {
		writeError(w, http.StatusForbidden, "managed routines cannot be triggered manually")
		return
	}
	if !routine.Enabled {
		writeError(w, http.StatusBadRequest, "routine is paused")
		return
	}
	triggers, err := h.Queries.ListRoutineTriggers(r.Context(), routine.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load routine triggers")
		return
	}
	var trigger db.RoutineTrigger
	for _, candidate := range triggers {
		if candidate.Enabled {
			trigger = candidate
			break
		}
	}
	if !trigger.ID.Valid {
		writeError(w, http.StatusBadRequest, "routine has no enabled triggers")
		return
	}
	actions, err := h.Queries.ListEnabledRoutineActions(r.Context(), routine.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load routine actions")
		return
	}
	if len(actions) == 0 {
		writeError(w, http.StatusBadRequest, "routine has no enabled actions")
		return
	}
	body := ""
	if routine.Instructions.Valid {
		body = routine.Instructions.String
	}
	payload, _ := json.Marshal(map[string]string{"manual": "true"})
	evt := service.RoutineEvent{
		Type:     "manual",
		DedupKey: "manual:" + uuidToString(routine.ID) + ":" + time.Now().UTC().Format(time.RFC3339Nano),
		Data: map[string]string{
			"title": routine.Name,
			"body":  body,
		},
		Payload: payload,
	}
	routineSvc := service.NewRoutineService(h.Queries, h.TxStarter, h.TaskService)
	ran := 0
	for _, action := range actions {
		result, err := routineSvc.ExecuteAction(r.Context(), routine, trigger, action, evt)
		if err != nil {
			slog.Warn("manual routine trigger action failed", "routine_id", uuidToString(routine.ID), "action_id", uuidToString(action.ID), "error", err)
			writeError(w, http.StatusInternalServerError, "failed to trigger routine")
			return
		}
		if result.Ran {
			ran++
		}
	}
	if ran > 0 {
		if _, err := h.Queries.IncrementRoutineTriggerSuccess(r.Context(), trigger.ID); err != nil {
			slog.Warn("manual routine trigger success increment failed", "trigger_id", uuidToString(trigger.ID), "error", err)
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"ran": ran})
}

func (h *Handler) RegenerateRoutineTriggerToken(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	triggerID := parseUUID(chi.URLParam(r, "triggerId"))
	trigger, err := h.Queries.GetRoutineTriggerInWorkspace(r.Context(), db.GetRoutineTriggerInWorkspaceParams{
		ID:          triggerID,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil || uuidToString(trigger.RoutineID) != chi.URLParam(r, "id") {
		writeError(w, http.StatusNotFound, "routine trigger not found")
		return
	}
	if routine, err := h.Queries.GetRoutine(r.Context(), trigger.RoutineID); err == nil && routine.Managed {
		writeError(w, http.StatusForbidden, "managed routine triggers cannot be modified")
		return
	}
	if trigger.TriggerType != "api" {
		writeError(w, http.StatusBadRequest, "only API routine triggers use tokens")
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
	updated, err := h.Queries.UpdateRoutineTrigger(r.Context(), db.UpdateRoutineTriggerParams{
		ID:                 trigger.ID,
		SourceType:         trigger.SourceType,
		TokenHash:          pgtype.Text{String: auth.HashToken(rawToken), Valid: true},
		TokenPrefix:        prefix,
		InstallationID:     trigger.InstallationID,
		Schedule:           trigger.Schedule,
		Timezone:           trigger.Timezone,
		RunAt:              trigger.RunAt,
		NextRunAt:          trigger.NextRunAt,
		DedupWindowSeconds: trigger.DedupWindowSeconds,
		MaxRuns:            trigger.MaxRuns,
		Config:             trigger.Config,
		Enabled:            trigger.Enabled,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to regenerate routine trigger token")
		return
	}

	writeJSON(w, http.StatusOK, RoutineTriggerTokenResponse{
		Trigger: routineTriggerToResponse(updated, rawToken),
		Token:   rawToken,
	})
}

func (h *Handler) GenerateRoutineTriggerTokenDraft(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
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
	draft, err := h.Queries.CreateRoutineTriggerTokenDraft(r.Context(), db.CreateRoutineTriggerTokenDraftParams{
		WorkspaceID: parseUUID(workspaceID),
		CreatedByID: member.UserID,
		TokenHash:   auth.HashToken(rawToken),
		TokenPrefix: prefix,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate routine trigger token")
		return
	}
	writeJSON(w, http.StatusCreated, RoutineTriggerTokenDraftResponse{
		DraftID:     uuidToString(draft.ID),
		TokenPrefix: draft.TokenPrefix,
		Token:       rawToken,
	})
}

func validateRoutineRequest(req CreateRoutineRequest) string {
	if strings.TrimSpace(req.Name) == "" {
		return "name is required"
	}
	if req.Instructions == nil || strings.TrimSpace(*req.Instructions) == "" {
		return "instructions is required"
	}
	if req.AssigneeType == nil || *req.AssigneeType == "" || req.AssigneeID == nil || *req.AssigneeID == "" {
		return "assignee is required"
	}
	if len(req.Triggers) == 0 {
		return "at least one trigger is required"
	}
	return ""
}

func (h *Handler) ListRoutineRuns(w http.ResponseWriter, r *http.Request) {
	workspaceID := ctxWorkspaceID(r.Context())
	id := parseUUID(chi.URLParam(r, "id"))
	if _, err := h.Queries.GetRoutineInWorkspace(r.Context(), db.GetRoutineInWorkspaceParams{ID: id, WorkspaceID: parseUUID(workspaceID)}); err != nil {
		writeError(w, http.StatusNotFound, "routine not found")
		return
	}
	runs, err := h.Queries.ListRoutineRuns(r.Context(), db.ListRoutineRunsParams{
		RoutineID: id,
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list routine runs")
		return
	}
	resp := make([]RoutineRunResponse, 0, len(runs))
	prefix := h.getIssuePrefix(r.Context(), parseUUID(workspaceID))
	for _, run := range runs {
		row := routineRunToResponse(run)
		if run.IssueID.Valid {
			if issue, err := h.Queries.GetIssue(r.Context(), run.IssueID); err == nil {
				issueResp := issueToResponse(issue, prefix)
				row.Issue = &issueResp
			}
		}
		resp = append(resp, row)
	}
	writeJSON(w, http.StatusOK, resp)
}

func routineRunToResponse(run db.RoutineRun) RoutineRunResponse {
	var payload any
	if len(run.Payload) > 0 {
		_ = json.Unmarshal(run.Payload, &payload)
	}
	return RoutineRunResponse{
		ID:           uuidToString(run.ID),
		RoutineID:    uuidToString(run.RoutineID),
		TriggerID:    uuidToPtr(run.TriggerID),
		ActionID:     uuidToPtr(run.ActionID),
		EventType:    run.EventType,
		DedupKey:     run.DedupKey,
		Payload:      payload,
		Status:       run.Status,
		IssueID:      uuidToPtr(run.IssueID),
		CommentID:    uuidToPtr(run.CommentID),
		ErrorMessage: textToPtr(run.ErrorMessage),
		CreatedAt:    timestampToString(run.CreatedAt),
	}
}

func (h *Handler) IngestRoutineTrigger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	authHeader := r.Header.Get("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == "" || tokenString == authHeader {
		writeError(w, http.StatusUnauthorized, "missing or invalid authorization")
		return
	}
	trigger, err := h.Queries.GetRoutineTriggerByTokenHash(r.Context(), pgtype.Text{String: auth.HashToken(tokenString), Valid: true})
	if err != nil || uuidToString(trigger.ID) != id {
		writeError(w, http.StatusUnauthorized, "invalid routine trigger token")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, routineTriggerMaxPayloadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	if len(body) > routineTriggerMaxPayloadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "payload exceeds 1MB limit")
		return
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	sourceType := "standard"
	if trigger.SourceType.Valid && trigger.SourceType.String != "" {
		sourceType = trigger.SourceType.String
	}
	received, ran := h.ingestRoutineTriggers(r.Context(), []db.RoutineTrigger{trigger}, sourceType, body, r.Header)
	writeJSON(w, http.StatusAccepted, map[string]any{"received": received, "ran": ran})
}

func (h *Handler) ingestRoutineTriggers(ctx context.Context, triggers []db.RoutineTrigger, sourceType string, body []byte, headers http.Header) (int, int) {
	if len(triggers) == 0 {
		return 0, 0
	}
	adapter, err := wh.GetAdapter(sourceType)
	if err != nil {
		return 0, 0
	}
	events, err := adapter.Parse(json.RawMessage(body), headers)
	if err != nil || len(events) == 0 {
		return 0, 0
	}
	routineSvc := service.NewRoutineService(h.Queries, h.TxStarter, h.TaskService)
	ran := 0
	for _, trigger := range triggers {
		routine, err := h.Queries.GetRoutine(ctx, trigger.RoutineID)
		if err != nil || !routine.Enabled || !trigger.Enabled {
			continue
		}
		actions, err := h.Queries.ListEnabledRoutineActions(ctx, routine.ID)
		if err != nil {
			continue
		}
		for _, evt := range events {
			if !routineTriggerMatchesEvent(trigger, evt) {
				continue
			}
			if evt.DedupKey != "" && trigger.DedupWindowSeconds > 0 {
				if _, err := h.Queries.FindRecentRoutineRun(ctx, db.FindRecentRoutineRunParams{
					TriggerID:     trigger.ID,
					DedupKey:      evt.DedupKey,
					WindowSeconds: float64(trigger.DedupWindowSeconds),
				}); err == nil {
					_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, pgtype.UUID{}, evt, "deduped", pgtype.UUID{}, pgtype.UUID{}, "")
					continue
				}
			}
			processed := false
			for _, action := range actions {
				if !routineActionMatchesEvent(action, evt) {
					continue
				}
				var didRun bool
				var execErr error
				if action.ActionType == "comment_issue" {
					// comment_issue runs on the Handler so it reuses the full
					// bot-comment + agent re-engagement pipeline.
					didRun, execErr = h.executeRoutineCommentIssue(ctx, routine, trigger, action, evt)
				} else {
					result, err := routineSvc.ExecuteAction(ctx, routine, trigger, action, service.RoutineEvent{
						Type:     evt.Type,
						DedupKey: evt.DedupKey,
						Data:     evt.Data,
						Payload:  evt.RawPayload,
					})
					didRun, execErr = result.Ran, err
				}
				if execErr != nil {
					_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, execErr.Error())
					continue
				}
				if didRun {
					processed = true
					ran++
				}
			}
			if !processed {
				_ = h.logRoutineRun(ctx, routine.ID, trigger.ID, pgtype.UUID{}, evt, "filtered", pgtype.UUID{}, pgtype.UUID{}, "no matching action")
			}
		}
	}
	return len(events), ran
}

func routineActionMatchesEvent(action db.RoutineAction, evt wh.Event) bool {
	switch action.ActionType {
	case "create_issue":
		var cfg CreateIssueActionConfig
		_ = json.Unmarshal(action.Config, &cfg)
		return actionMatchesFilters(cfg.EventTypes, cfg.Repos, cfg.GitHubLabels, evt)
	case "comment_issue":
		var cfg CommentIssueActionConfig
		_ = json.Unmarshal(action.Config, &cfg)
		return actionMatchesFilters(cfg.EventTypes, cfg.Repos, cfg.GitHubLabels, evt)
	default:
		return false
	}
}

type routineGitHubTriggerConfig struct {
	EventTypes []string               `json:"event_types"`
	Filters    []routineTriggerFilter `json:"filters"`
}

type routineTriggerFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

func routineTriggerMatchesEvent(trigger db.RoutineTrigger, evt wh.Event) bool {
	if trigger.TriggerType != "github" || len(trigger.Config) == 0 {
		return true
	}
	var cfg routineGitHubTriggerConfig
	if err := json.Unmarshal(trigger.Config, &cfg); err != nil {
		return false
	}
	if !routineTriggerEventTypesMatch(cfg.EventTypes, evt.Type) {
		return false
	}
	for _, filter := range cfg.Filters {
		if strings.TrimSpace(filter.Value) == "" {
			continue
		}
		if !routineTriggerFilterMatches(evt.Data[filter.Field], filter.Operator, filter.Value) {
			return false
		}
	}
	return true
}

func routineTriggerEventTypesMatch(eventTypes []string, eventType string) bool {
	if len(eventTypes) == 0 {
		return true
	}
	for _, want := range eventTypes {
		want = strings.TrimSpace(want)
		if want == "" {
			continue
		}
		if want == eventType || strings.HasPrefix(eventType, want+".") {
			return true
		}
		if want == "github.pull_request.closed" && eventType == "github.pull_request.merged" {
			return true
		}
	}
	return false
}

func routineTriggerFilterMatches(actual, operator, expected string) bool {
	operator = strings.TrimSpace(strings.ToLower(operator))
	if operator == "" {
		operator = "is one of"
	}
	actual = strings.TrimSpace(actual)
	expected = strings.TrimSpace(expected)
	switch operator {
	case "equals":
		return actual == expected
	case "contains":
		return strings.Contains(actual, expected)
	case "starts with":
		return strings.HasPrefix(actual, expected)
	case "matches regex (whole string)":
		re, err := regexp.Compile("^(?:" + expected + ")$")
		return err == nil && re.MatchString(actual)
	case "is not one of":
		return !routineTriggerValueInList(actual, expected)
	case "is one of":
		return routineTriggerValueInList(actual, expected)
	default:
		return routineTriggerValueInList(actual, expected)
	}
}

func routineTriggerValueInList(actual, expected string) bool {
	actualValues := splitRoutineTriggerFilterValues(actual)
	expectedValues := splitRoutineTriggerFilterValues(expected)
	for _, actualValue := range actualValues {
		for _, expectedValue := range expectedValues {
			if actualValue == expectedValue {
				return true
			}
		}
	}
	return false
}

func splitRoutineTriggerFilterValues(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (h *Handler) logRoutineRun(ctx context.Context, routineID, triggerID, actionID pgtype.UUID, evt wh.Event, status string, issueID, commentID pgtype.UUID, errorMessage string) error {
	errText := pgtype.Text{}
	if errorMessage != "" {
		errText = pgtype.Text{String: errorMessage, Valid: true}
	}
	_, err := h.Queries.CreateRoutineRun(ctx, db.CreateRoutineRunParams{
		RoutineID:    routineID,
		TriggerID:    triggerID,
		ActionID:     actionID,
		EventType:    evt.Type,
		DedupKey:     evt.DedupKey,
		Payload:      evt.RawPayload,
		Status:       status,
		IssueID:      issueID,
		CommentID:    commentID,
		ErrorMessage: errText,
	})
	return err
}

func (h *Handler) saveRoutine(ctx context.Context, workspaceID pgtype.UUID, createdByID pgtype.UUID, createdByType string, req CreateRoutineRequest, existingID *pgtype.UUID) (db.Routine, map[string]string, error) {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return db.Routine{}, nil, err
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)

	priority := "medium"
	if req.Priority != nil && *req.Priority != "" {
		priority = *req.Priority
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	githubAutoFixEnabled := false
	if req.GithubAutoFixEnabled != nil {
		githubAutoFixEnabled = *req.GithubAutoFixEnabled
	}

	var routine db.Routine
	if existingID == nil {
		routine, err = qtx.CreateRoutine(ctx, db.CreateRoutineParams{
			WorkspaceID:          workspaceID,
			Name:                 req.Name,
			Instructions:         ptrToText(req.Instructions),
			Priority:             priority,
			AssigneeType:         ptrToText(req.AssigneeType),
			AssigneeID:           parseOptionalUUID(req.AssigneeID),
			DueDateOffsetHours:   optionalInt4(req.DueDateOffsetHours),
			DispatchProvider:     ptrToText(req.DispatchProvider),
			DispatchDaemonID:     parseOptionalUUID(req.DispatchDaemonID),
			DispatchDaemonLabel:  ptrToText(req.DispatchDaemonLabel),
			Enabled:              enabled,
			GithubAutoFixEnabled: githubAutoFixEnabled,
			CreatedByID:          createdByID,
			CreatedByType:        createdByType,
		})
	} else {
		routine, err = qtx.UpdateRoutine(ctx, db.UpdateRoutineParams{
			ID:                   *existingID,
			Name:                 req.Name,
			Instructions:         ptrToText(req.Instructions),
			Priority:             priority,
			AssigneeType:         ptrToText(req.AssigneeType),
			AssigneeID:           parseOptionalUUID(req.AssigneeID),
			DueDateOffsetHours:   optionalInt4(req.DueDateOffsetHours),
			DispatchProvider:     ptrToText(req.DispatchProvider),
			DispatchDaemonID:     parseOptionalUUID(req.DispatchDaemonID),
			DispatchDaemonLabel:  ptrToText(req.DispatchDaemonLabel),
			Enabled:              enabled,
			GithubAutoFixEnabled: githubAutoFixEnabled,
			WorkspaceID:          workspaceID,
		})
		if err == nil {
			actions, _ := qtx.ListRoutineActions(ctx, *existingID)
			for _, action := range actions {
				_ = qtx.DeleteRoutineAction(ctx, action.ID)
			}
			_ = qtx.ClearRoutineLabels(ctx, *existingID)
			_ = qtx.ClearRoutineSubscribers(ctx, *existingID)
		}
	}
	if err != nil {
		return db.Routine{}, nil, err
	}
	existingTriggers := []db.RoutineTrigger{}
	if existingID != nil {
		existingTriggers, _ = qtx.ListRoutineTriggers(ctx, *existingID)
	}
	existingTriggersByID := make(map[string]db.RoutineTrigger, len(existingTriggers))
	for _, trigger := range existingTriggers {
		existingTriggersByID[uuidToString(trigger.ID)] = trigger
	}

	for _, labelID := range req.LabelIDs {
		lid := parseUUID(labelID)
		if lid.Valid {
			_ = qtx.AddRoutineLabel(ctx, db.AddRoutineLabelParams{RoutineID: routine.ID, LabelID: lid})
		}
	}
	for _, subID := range req.SubscriberIDs {
		uid := parseUUID(subID)
		if uid.Valid {
			_ = qtx.AddRoutineSubscriber(ctx, db.AddRoutineSubscriberParams{RoutineID: routine.ID, UserID: uid})
		}
	}

	triggerTokens := map[string]string{}
	keptTriggerIDs := map[string]bool{}
	for _, triggerReq := range req.Triggers {
		var existingTrigger db.RoutineTrigger
		hasExistingTrigger := false
		if triggerReq.ID != nil {
			if candidate, ok := existingTriggersByID[*triggerReq.ID]; ok {
				existingTrigger = candidate
				hasExistingTrigger = true
			}
		}
		trigger, token, err := saveRoutineTrigger(ctx, qtx, workspaceID, routine.ID, triggerReq, existingTrigger, hasExistingTrigger)
		if err != nil {
			return db.Routine{}, nil, err
		}
		keptTriggerIDs[uuidToString(trigger.ID)] = true
		if token != "" {
			triggerTokens[uuidToString(trigger.ID)] = token
		}
	}
	for _, trigger := range existingTriggers {
		if !keptTriggerIDs[uuidToString(trigger.ID)] {
			_ = qtx.DeleteRoutineTrigger(ctx, trigger.ID)
		}
	}
	actions := req.Actions
	if len(actions) == 0 {
		actions = []RoutineActionRequest{{ActionType: "create_issue", Config: map[string]any{}}}
	}
	for i, actionReq := range actions {
		configJSON, err := json.Marshal(actionReq.Config)
		if err != nil {
			return db.Routine{}, nil, err
		}
		actionType := actionReq.ActionType
		if actionType == "" {
			actionType = "create_issue"
		}
		actionEnabled := true
		if actionReq.Enabled != nil {
			actionEnabled = *actionReq.Enabled
		}
		position := int32(i)
		if actionReq.Position != nil {
			position = *actionReq.Position
		}
		if _, err := qtx.CreateRoutineAction(ctx, db.CreateRoutineActionParams{
			RoutineID:  routine.ID,
			ActionType: actionType,
			Config:     configJSON,
			Enabled:    actionEnabled,
			Position:   position,
		}); err != nil {
			return db.Routine{}, nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Routine{}, nil, err
	}
	return routine, triggerTokens, nil
}

func saveRoutineTrigger(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, routineID pgtype.UUID, req RoutineTriggerRequest, existing db.RoutineTrigger, hasExisting bool) (db.RoutineTrigger, string, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	dedupWindow := int32(600)
	if req.DedupWindowSeconds != nil {
		dedupWindow = *req.DedupWindowSeconds
	}
	timezone := "UTC"
	if req.Timezone != nil && *req.Timezone != "" {
		timezone = *req.Timezone
	}
	var runAt pgtype.Timestamptz
	if req.RunAt != nil && *req.RunAt != "" {
		if t, err := time.Parse(time.RFC3339, *req.RunAt); err == nil {
			runAt = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	var nextRunAt pgtype.Timestamptz
	if enabled && req.TriggerType == "schedule" {
		if runAt.Valid {
			nextRunAt = runAt
		} else if req.Schedule != nil && *req.Schedule != "" {
			loc, err := time.LoadLocation(timezone)
			if err != nil {
				return db.RoutineTrigger{}, "", err
			}
			sched, err := cron.Parse(*req.Schedule)
			if err != nil {
				return db.RoutineTrigger{}, "", err
			}
			if next := sched.Next(time.Now().In(loc)); !next.IsZero() {
				nextRunAt = pgtype.Timestamptz{Time: next, Valid: true}
			}
		}
	}
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return db.RoutineTrigger{}, "", err
	}
	var tokenHash pgtype.Text
	tokenPrefix := ""
	var triggerID pgtype.UUID
	if req.TriggerType == "api" {
		if req.TokenDraftID != nil && *req.TokenDraftID != "" {
			draftID := parseUUID(*req.TokenDraftID)
			if !draftID.Valid {
				return db.RoutineTrigger{}, "", errors.New("invalid routine trigger token draft")
			}
			draft, err := q.ConsumeRoutineTriggerTokenDraft(ctx, db.ConsumeRoutineTriggerTokenDraftParams{
				ID:          draftID,
				WorkspaceID: workspaceID,
			})
			if err != nil {
				return db.RoutineTrigger{}, "", errRoutineTriggerTokenRequired
			}
			triggerID = draft.ID
			tokenHash = pgtype.Text{String: draft.TokenHash, Valid: true}
			tokenPrefix = draft.TokenPrefix
		} else if hasExisting && existing.TriggerType == "api" {
			triggerID = existing.ID
			tokenHash = existing.TokenHash
			tokenPrefix = existing.TokenPrefix
		} else {
			return db.RoutineTrigger{}, "", errRoutineTriggerTokenRequired
		}
	} else if hasExisting {
		triggerID = existing.ID
	}
	var installationID pgtype.Int8
	if req.TriggerType == "github" {
		workspace, err := q.GetWorkspace(ctx, workspaceID)
		if err != nil {
			return db.RoutineTrigger{}, "", err
		}
		installationID = workspace.GithubInstallationID
	}
	if hasExisting {
		trigger, err := q.UpdateRoutineTrigger(ctx, db.UpdateRoutineTriggerParams{
			ID:                 existing.ID,
			SourceType:         ptrToText(req.SourceType),
			TokenHash:          tokenHash,
			TokenPrefix:        tokenPrefix,
			InstallationID:     installationID,
			Schedule:           ptrToText(req.Schedule),
			Timezone:           timezone,
			RunAt:              runAt,
			NextRunAt:          nextRunAt,
			DedupWindowSeconds: dedupWindow,
			MaxRuns:            optionalInt4(req.MaxRuns),
			Config:             configJSON,
			Enabled:            enabled,
		})
		return trigger, "", err
	}
	if triggerID.Valid {
		trigger, err := q.CreateRoutineTriggerWithID(ctx, db.CreateRoutineTriggerWithIDParams{
			ID:                 triggerID,
			RoutineID:          routineID,
			TriggerType:        req.TriggerType,
			SourceType:         ptrToText(req.SourceType),
			TokenHash:          tokenHash,
			TokenPrefix:        tokenPrefix,
			InstallationID:     installationID,
			Schedule:           ptrToText(req.Schedule),
			Timezone:           timezone,
			RunAt:              runAt,
			NextRunAt:          nextRunAt,
			DedupWindowSeconds: dedupWindow,
			MaxRuns:            optionalInt4(req.MaxRuns),
			Config:             configJSON,
			Enabled:            enabled,
		})
		return trigger, "", err
	}
	trigger, err := q.CreateRoutineTrigger(ctx, db.CreateRoutineTriggerParams{
		RoutineID:          routineID,
		TriggerType:        req.TriggerType,
		SourceType:         ptrToText(req.SourceType),
		TokenHash:          tokenHash,
		TokenPrefix:        tokenPrefix,
		InstallationID:     installationID,
		Schedule:           ptrToText(req.Schedule),
		Timezone:           timezone,
		RunAt:              runAt,
		NextRunAt:          nextRunAt,
		DedupWindowSeconds: dedupWindow,
		MaxRuns:            optionalInt4(req.MaxRuns),
		Config:             configJSON,
		Enabled:            enabled,
	})
	return trigger, "", err
}

func (h *Handler) buildRoutineResponse(ctx context.Context, routine db.Routine, triggerTokens map[string]string) (RoutineResponse, error) {
	triggers, err := h.Queries.ListRoutineTriggers(ctx, routine.ID)
	if err != nil {
		return RoutineResponse{}, err
	}
	triggerResponses := make([]RoutineTriggerResponse, len(triggers))
	for i, trigger := range triggers {
		token := ""
		if triggerTokens != nil {
			token = triggerTokens[uuidToString(trigger.ID)]
		}
		triggerResponses[i] = routineTriggerToResponse(trigger, token)
	}
	actions, err := h.Queries.ListRoutineActions(ctx, routine.ID)
	if err != nil {
		return RoutineResponse{}, err
	}
	actionResponses := make([]RoutineActionResponse, len(actions))
	for i, action := range actions {
		actionResponses[i] = routineActionToResponse(action)
	}
	subs, _ := h.Queries.ListRoutineSubscribers(ctx, routine.ID)
	subscriberIDs := make([]string, len(subs))
	for i, sub := range subs {
		subscriberIDs[i] = uuidToString(sub.UserID)
	}
	labels, _ := h.Queries.ListRoutineLabels(ctx, routine.ID)
	labelIDs := make([]string, len(labels))
	for i, label := range labels {
		labelIDs[i] = uuidToString(label.LabelID)
	}
	return routineToResponse(routine, subscriberIDs, labelIDs, triggerResponses, actionResponses), nil
}

func optionalInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}
