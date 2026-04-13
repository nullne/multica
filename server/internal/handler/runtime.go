package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

type DaemonResponse struct {
	ID          string   `json:"id"`
	WorkspaceID string   `json:"workspace_id"`
	DaemonID    string   `json:"daemon_id"`
	Status      string   `json:"status"`
	CLIVersion  string   `json:"cli_version"`
	DeviceName  string   `json:"device_name"`
	DeviceInfo  string   `json:"device_info"`
	Labels      []string `json:"labels"`
	Metadata    any      `json:"metadata"`
	LastSeenAt  *string  `json:"last_seen_at"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

func daemonToResponse(d db.Daemon) DaemonResponse {
	var metadata any
	if d.Metadata != nil {
		json.Unmarshal(d.Metadata, &metadata)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	labels := d.Labels
	if labels == nil {
		labels = []string{}
	}
	return DaemonResponse{
		ID:          uuidToString(d.ID),
		WorkspaceID: uuidToString(d.WorkspaceID),
		DaemonID:    d.DaemonID,
		Status:      d.Status,
		CLIVersion:  d.CliVersion,
		DeviceName:  d.DeviceName,
		DeviceInfo:  d.DeviceInfo,
		Labels:      labels,
		Metadata:    metadata,
		LastSeenAt:  timestampToPtr(d.LastSeenAt),
		CreatedAt:   timestampToString(d.CreatedAt),
		UpdatedAt:   timestampToString(d.UpdatedAt),
	}
}

func (h *Handler) UpdateDaemon(w http.ResponseWriter, r *http.Request) {
	daemonID := chi.URLParam(r, "daemonId")

	d, err := h.Queries.GetDaemon(r.Context(), parseUUID(daemonID))
	if err != nil {
		writeError(w, http.StatusNotFound, "daemon not found")
		return
	}

	if _, ok := h.requireWorkspaceMember(w, r, uuidToString(d.WorkspaceID), "daemon not found"); !ok {
		return
	}

	var req struct {
		DeviceName *string  `json:"device_name"`
		Labels     []string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateDaemonFieldsParams{ID: parseUUID(daemonID)}
	if req.DeviceName != nil {
		params.DeviceName = pgtype.Text{String: *req.DeviceName, Valid: true}
	}
	if req.Labels != nil {
		params.Labels = req.Labels
	}

	updated, err := h.Queries.UpdateDaemonFields(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update daemon")
		return
	}

	writeJSON(w, http.StatusOK, daemonToResponse(updated))
}

type AgentRuntimeResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	DaemonID    *string `json:"daemon_id"`
	DaemonRef   *string `json:"daemon_ref"`
	Name        string  `json:"name"`
	RuntimeMode string  `json:"runtime_mode"`
	Provider    string  `json:"provider"`
	Status      string  `json:"status"`
	DeviceInfo  string  `json:"device_info"`
	Metadata    any     `json:"metadata"`
	LastSeenAt  *string `json:"last_seen_at"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func runtimeToResponse(rt db.AgentRuntime) AgentRuntimeResponse {
	var metadata any
	if rt.Metadata != nil {
		json.Unmarshal(rt.Metadata, &metadata)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}

	return AgentRuntimeResponse{
		ID:          uuidToString(rt.ID),
		WorkspaceID: uuidToString(rt.WorkspaceID),
		DaemonID:    textToPtr(rt.DaemonID),
		DaemonRef:   uuidToPtr(rt.DaemonRef),
		Name:        rt.Name,
		RuntimeMode: rt.RuntimeMode,
		Provider:    rt.Provider,
		Status:      rt.Status,
		DeviceInfo:  rt.DeviceInfo,
		Metadata:    metadata,
		LastSeenAt:  timestampToPtr(rt.LastSeenAt),
		CreatedAt:   timestampToString(rt.CreatedAt),
		UpdatedAt:   timestampToString(rt.UpdatedAt),
	}
}

// ---------------------------------------------------------------------------
// Runtime Usage
// ---------------------------------------------------------------------------

type RuntimeUsageEntry struct {
	Date             string `json:"date"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
}

type RuntimeUsageResponse struct {
	RuntimeID        string `json:"runtime_id"`
	Date             string `json:"date"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
}

// ReportRuntimeUsage receives usage data from the daemon (unauthenticated daemon route).
func (h *Handler) ReportRuntimeUsage(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	if runtimeID == "" {
		writeError(w, http.StatusBadRequest, "runtimeId is required")
		return
	}

	var req struct {
		Entries []RuntimeUsageEntry `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, entry := range req.Entries {
		date, err := time.Parse("2006-01-02", entry.Date)
		if err != nil {
			continue
		}
		h.Queries.UpsertRuntimeUsage(r.Context(), db.UpsertRuntimeUsageParams{
			RuntimeID:        parseUUID(runtimeID),
			Date:             pgtype.Date{Time: date, Valid: true},
			Provider:         entry.Provider,
			Model:            entry.Model,
			InputTokens:      entry.InputTokens,
			OutputTokens:     entry.OutputTokens,
			CacheReadTokens:  entry.CacheReadTokens,
			CacheWriteTokens: entry.CacheWriteTokens,
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetRuntimeUsage returns usage data for a runtime (protected route).
func (h *Handler) GetRuntimeUsage(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")

	rt, err := h.Queries.GetAgentRuntime(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusNotFound, "runtime not found")
		return
	}

	if _, ok := h.requireWorkspaceMember(w, r, uuidToString(rt.WorkspaceID), "runtime not found"); !ok {
		return
	}

	limit := int32(90)
	if l := r.URL.Query().Get("days"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 365 {
			limit = int32(parsed)
		}
	}

	rows, err := h.Queries.ListRuntimeUsage(r.Context(), db.ListRuntimeUsageParams{
		RuntimeID: parseUUID(runtimeID),
		Limit:     limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list usage")
		return
	}

	resp := make([]RuntimeUsageResponse, len(rows))
	for i, row := range rows {
		resp[i] = RuntimeUsageResponse{
			RuntimeID:        runtimeID,
			Date:             row.Date.Time.Format("2006-01-02"),
			Provider:         row.Provider,
			Model:            row.Model,
			InputTokens:      row.InputTokens,
			OutputTokens:     row.OutputTokens,
			CacheReadTokens:  row.CacheReadTokens,
			CacheWriteTokens: row.CacheWriteTokens,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetRuntimeTaskActivity returns hourly task activity distribution for a runtime.
func (h *Handler) GetRuntimeTaskActivity(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")

	rt, err := h.Queries.GetAgentRuntime(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusNotFound, "runtime not found")
		return
	}

	if _, ok := h.requireWorkspaceMember(w, r, uuidToString(rt.WorkspaceID), "runtime not found"); !ok {
		return
	}

	rows, err := h.Queries.GetRuntimeTaskHourlyActivity(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get task activity")
		return
	}

	type HourlyActivity struct {
		Hour  int `json:"hour"`
		Count int `json:"count"`
	}

	resp := make([]HourlyActivity, len(rows))
	for i, row := range rows {
		resp[i] = HourlyActivity{Hour: int(row.Hour), Count: int(row.Count)}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListDaemons(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	daemons, err := h.Queries.ListDaemons(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list daemons")
		return
	}

	resp := make([]DaemonResponse, len(daemons))
	for i, d := range daemons {
		resp[i] = daemonToResponse(d)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListAgentRuntimes(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	runtimes, err := h.Queries.ListAgentRuntimes(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runtimes")
		return
	}

	resp := make([]AgentRuntimeResponse, len(runtimes))
	for i, rt := range runtimes {
		resp[i] = runtimeToResponse(rt)
	}

	writeJSON(w, http.StatusOK, resp)
}
