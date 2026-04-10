package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	internalcli "github.com/nullne/multica/server/internal/cli"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// In-memory update store
// ---------------------------------------------------------------------------

type UpdateStatus string

const (
	UpdatePending   UpdateStatus = "pending"
	UpdateRunning   UpdateStatus = "running"
	UpdateCompleted UpdateStatus = "completed"
	UpdateFailed    UpdateStatus = "failed"
	UpdateTimeout   UpdateStatus = "timeout"
)

// UpdateRequest represents a pending or completed CLI update request.
type UpdateRequest struct {
	ID            string       `json:"id"`
	RuntimeID     string       `json:"runtime_id"`
	Status        UpdateStatus `json:"status"`
	TargetVersion string       `json:"target_version"`
	UpdateClients bool         `json:"update_clients"`
	Output        string       `json:"output,omitempty"`
	Error         string       `json:"error,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// UpdateStore is a thread-safe in-memory store for CLI update requests.
type UpdateStore struct {
	mu              sync.Mutex
	requests        map[string]*UpdateRequest // keyed by update ID
	autoAttempts    map[string]time.Time      // runtimeID::targetVersion -> attempt time
	latestVersion   string
	latestVersionAt time.Time
}

func NewUpdateStore() *UpdateStore {
	return &UpdateStore{
		requests:     make(map[string]*UpdateRequest),
		autoAttempts: make(map[string]time.Time),
	}
}

func (s *UpdateStore) Create(runtimeID, targetVersion string, updateClients bool) (*UpdateRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked()
	if s.hasInProgressLocked(runtimeID) {
		return nil, errUpdateInProgress
	}
	req := s.createLocked(runtimeID, targetVersion, updateClients)
	return req, nil
}

// CreateAuto creates an automatic update request and throttles repeated
// attempts for the same runtime/version pair.
func (s *UpdateStore) CreateAuto(runtimeID, targetVersion string, updateClients bool) (*UpdateRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked()
	if s.hasInProgressLocked(runtimeID) {
		return nil, errUpdateInProgress
	}
	key := runtimeID + "::" + targetVersion
	if last, ok := s.autoAttempts[key]; ok && time.Since(last) < 30*time.Minute {
		return nil, nil
	}
	req := s.createLocked(runtimeID, targetVersion, updateClients)
	s.autoAttempts[key] = time.Now()
	return req, nil
}

var errUpdateInProgress = &updateError{msg: "an update is already in progress for this runtime"}

type updateError struct{ msg string }

func (e *updateError) Error() string { return e.msg }

func (s *UpdateStore) cleanupLocked() {
	for id, req := range s.requests {
		if time.Since(req.CreatedAt) > 5*time.Minute {
			delete(s.requests, id)
		}
	}
	for key, at := range s.autoAttempts {
		if time.Since(at) > 30*time.Minute {
			delete(s.autoAttempts, key)
		}
	}
}

func (s *UpdateStore) hasInProgressLocked(runtimeID string) bool {
	for _, req := range s.requests {
		if req.RuntimeID == runtimeID && (req.Status == UpdatePending || req.Status == UpdateRunning) {
			return true
		}
	}
	return false
}

func (s *UpdateStore) createLocked(runtimeID, targetVersion string, updateClients bool) *UpdateRequest {
	req := &UpdateRequest{
		ID:            randomID(),
		RuntimeID:     runtimeID,
		Status:        UpdatePending,
		TargetVersion: targetVersion,
		UpdateClients: updateClients,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	s.requests[req.ID] = req
	return req
}

func (s *UpdateStore) ResolveLatestVersion(fetch func() (string, error)) (string, error) {
	s.mu.Lock()
	if s.latestVersion != "" && time.Since(s.latestVersionAt) < 10*time.Minute {
		tag := s.latestVersion
		s.mu.Unlock()
		return tag, nil
	}
	s.mu.Unlock()

	tag, err := fetch()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.latestVersion = tag
	s.latestVersionAt = time.Now()
	s.mu.Unlock()
	return tag, nil
}

func (s *UpdateStore) Get(id string) *UpdateRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[id]
	if !ok {
		return nil
	}
	// Check for timeout (both pending and running states).
	if (req.Status == UpdatePending || req.Status == UpdateRunning) && time.Since(req.CreatedAt) > 120*time.Second {
		req.Status = UpdateTimeout
		req.Error = "update did not complete within 120 seconds"
		req.UpdatedAt = time.Now()
	}
	return req
}

// PopPending returns and marks as running the pending update for a runtime.
func (s *UpdateStore) PopPending(runtimeID string) *UpdateRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, req := range s.requests {
		if req.RuntimeID == runtimeID && req.Status == UpdatePending {
			req.Status = UpdateRunning
			req.UpdatedAt = time.Now()
			return req
		}
	}
	return nil
}

func (s *UpdateStore) Complete(id string, output string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req, ok := s.requests[id]; ok {
		req.Status = UpdateCompleted
		req.Output = output
		req.UpdatedAt = time.Now()
	}
}

func (s *UpdateStore) Fail(id string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req, ok := s.requests[id]; ok {
		req.Status = UpdateFailed
		req.Error = errMsg
		req.UpdatedAt = time.Now()
	}
}

type runtimeAutoUpdateSettings struct {
	Enabled       bool
	TargetVersion string
	UpdateClients bool
}

func parseRuntimeAutoUpdateSettings(rawSettings []byte) runtimeAutoUpdateSettings {
	settings := runtimeAutoUpdateSettings{
		Enabled:       false,
		TargetVersion: "latest",
		UpdateClients: true,
	}
	if len(rawSettings) == 0 {
		return settings
	}
	var root map[string]any
	if err := json.Unmarshal(rawSettings, &root); err != nil {
		return settings
	}
	val, ok := root["runtime_auto_update"]
	if !ok {
		return settings
	}
	switch typed := val.(type) {
	case bool:
		settings.Enabled = typed
	case map[string]any:
		settings.Enabled = true
		if v, ok := boolSetting(typed["enabled"]); ok {
			settings.Enabled = v
		}
		if v, ok := stringSetting(typed["target_version"]); ok && strings.TrimSpace(v) != "" {
			settings.TargetVersion = strings.TrimSpace(v)
		}
		if v, ok := boolSetting(typed["update_clients"]); ok {
			settings.UpdateClients = v
		}
	}
	return settings
}

func boolSetting(v any) (bool, bool) {
	switch typed := v.(type) {
	case bool:
		return typed, true
	case string:
		if parsed, err := strconv.ParseBool(strings.TrimSpace(typed)); err == nil {
			return parsed, true
		}
	}
	return false, false
}

func stringSetting(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func runtimeCLIVersion(metadata []byte) string {
	if len(metadata) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(metadata, &payload); err != nil {
		return ""
	}
	raw, ok := payload["cli_version"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(raw)
}

func shouldAutoUpdate(currentVersion, targetVersion string) bool {
	current := parseVersion(currentVersion)
	target := parseVersion(targetVersion)
	if len(target) == 0 {
		return false
	}
	if len(current) == 0 {
		return true
	}
	maxLen := len(current)
	if len(target) > maxLen {
		maxLen = len(target)
	}
	for i := 0; i < maxLen; i++ {
		cur := 0
		if i < len(current) {
			cur = current[i]
		}
		tar := 0
		if i < len(target) {
			tar = target[i]
		}
		if tar > cur {
			return true
		}
		if tar < cur {
			return false
		}
	}
	return false
}

func parseVersion(raw string) []int {
	s := strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	if s == "" {
		return nil
	}
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		s = s[:idx]
	}
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		out = append(out, leadingDigits(part))
	}
	return out
}

func leadingDigits(part string) int {
	part = strings.TrimSpace(part)
	if part == "" {
		return 0
	}
	n := 0
	for _, r := range part {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func (h *Handler) maybeCreateAutoUpdate(r *http.Request, rt db.AgentRuntime) *UpdateRequest {
	ws, err := h.Queries.GetWorkspace(r.Context(), rt.WorkspaceID)
	if err != nil {
		return nil
	}
	cfg := parseRuntimeAutoUpdateSettings(ws.Settings)
	if !cfg.Enabled {
		return nil
	}

	targetVersion := strings.TrimSpace(cfg.TargetVersion)
	if targetVersion == "" || strings.EqualFold(targetVersion, "latest") {
		resolved, err := h.UpdateStore.ResolveLatestVersion(func() (string, error) {
			latest, err := internalcli.FetchLatestRelease()
			if err != nil {
				return "", err
			}
			if latest == nil || strings.TrimSpace(latest.TagName) == "" {
				return "", fmt.Errorf("latest release tag is empty")
			}
			return latest.TagName, nil
		})
		if err != nil {
			slog.Warn("auto update: failed to resolve latest version", "runtime_id", uuidToString(rt.ID), "error", err)
			return nil
		}
		targetVersion = resolved
	}

	currentVersion := runtimeCLIVersion(rt.Metadata)
	if !shouldAutoUpdate(currentVersion, targetVersion) {
		return nil
	}

	update, err := h.UpdateStore.CreateAuto(uuidToString(rt.ID), targetVersion, cfg.UpdateClients)
	if err != nil {
		if err != errUpdateInProgress {
			slog.Warn("auto update: failed to create request", "runtime_id", uuidToString(rt.ID), "error", err)
		}
		return nil
	}
	if update != nil {
		slog.Info("auto update queued", "runtime_id", uuidToString(rt.ID), "current_version", currentVersion, "target_version", targetVersion, "update_clients", cfg.UpdateClients)
	}
	return update
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// InitiateUpdate creates a new CLI update request (protected route, called by frontend).
func (h *Handler) InitiateUpdate(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")

	rt, err := h.Queries.GetAgentRuntime(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusNotFound, "runtime not found")
		return
	}

	if _, ok := h.requireWorkspaceMember(w, r, uuidToString(rt.WorkspaceID), "runtime not found"); !ok {
		return
	}

	var req struct {
		TargetVersion string `json:"target_version"`
		UpdateClients *bool  `json:"update_clients"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TargetVersion == "" {
		writeError(w, http.StatusBadRequest, "target_version is required")
		return
	}

	updateClients := true
	if req.UpdateClients != nil {
		updateClients = *req.UpdateClients
	}
	update, err := h.UpdateStore.Create(runtimeID, req.TargetVersion, updateClients)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, update)
}

// GetUpdate returns the status of an update request (protected route, called by frontend).
func (h *Handler) GetUpdate(w http.ResponseWriter, r *http.Request) {
	updateID := chi.URLParam(r, "updateId")

	update := h.UpdateStore.Get(updateID)
	if update == nil {
		writeError(w, http.StatusNotFound, "update not found")
		return
	}

	writeJSON(w, http.StatusOK, update)
}

// ReportUpdateResult receives the update result from the daemon.
func (h *Handler) ReportUpdateResult(w http.ResponseWriter, r *http.Request) {
	updateID := chi.URLParam(r, "updateId")

	var req struct {
		Status string `json:"status"` // "running", "completed", or "failed"
		Output string `json:"output"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Status {
	case "completed":
		h.UpdateStore.Complete(updateID, req.Output)
	case "failed":
		h.UpdateStore.Fail(updateID, req.Error)
	case "running":
		// No-op: status is already "running" from PopPending. This call is
		// just a progress signal from the daemon to confirm it received the
		// update command and is executing it.
	default:
		writeError(w, http.StatusBadRequest, "invalid status: "+req.Status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
