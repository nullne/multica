package handler

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
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
	DaemonID      string       `json:"daemon_id"`
	RuntimeID     string       `json:"runtime_id,omitempty"` // deprecated: use DaemonID
	Target        string       `json:"target"`               // "multica", "claude", "codex", etc.
	Status        UpdateStatus `json:"status"`
	TargetVersion string       `json:"target_version"`
	Output        string       `json:"output,omitempty"`
	Error         string       `json:"error,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// UpdateStore is a thread-safe in-memory store for CLI update requests.
type UpdateStore struct {
	mu       sync.Mutex
	requests map[string]*UpdateRequest // keyed by update ID
}

func NewUpdateStore() *UpdateStore {
	return &UpdateStore{
		requests: make(map[string]*UpdateRequest),
	}
}

func (s *UpdateStore) Create(runtimeID, targetVersion string) (*UpdateRequest, error) {
	return s.CreateWithTarget(runtimeID, "multica", targetVersion)
}

// CreateForDaemon creates an update request keyed by daemon UUID.
func (s *UpdateStore) CreateForDaemon(daemonID, target, targetVersion string) (*UpdateRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, req := range s.requests {
		if time.Since(req.CreatedAt) > 5*time.Minute {
			delete(s.requests, id)
		}
	}

	for _, req := range s.requests {
		if req.DaemonID == daemonID && req.Target == target && (req.Status == UpdatePending || req.Status == UpdateRunning) {
			return nil, errUpdateInProgress
		}
	}

	req := &UpdateRequest{
		ID:            randomID(),
		DaemonID:      daemonID,
		Target:        target,
		Status:        UpdatePending,
		TargetVersion: targetVersion,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	s.requests[req.ID] = req
	return req, nil
}

func (s *UpdateStore) CreateWithTarget(runtimeID, target, targetVersion string) (*UpdateRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, req := range s.requests {
		if time.Since(req.CreatedAt) > 5*time.Minute {
			delete(s.requests, id)
		}
	}

	for _, req := range s.requests {
		if req.RuntimeID == runtimeID && req.Target == target && (req.Status == UpdatePending || req.Status == UpdateRunning) {
			return nil, errUpdateInProgress
		}
	}

	req := &UpdateRequest{
		ID:            randomID(),
		RuntimeID:     runtimeID,
		Target:        target,
		Status:        UpdatePending,
		TargetVersion: targetVersion,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	s.requests[req.ID] = req
	return req, nil
}

var errUpdateInProgress = &updateError{msg: "an update is already in progress for this runtime"}

type updateError struct{ msg string }

func (e *updateError) Error() string { return e.msg }

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

// PopPending returns and marks as running the first pending update for a runtime or daemon.
func (s *UpdateStore) PopPending(id string) *UpdateRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, req := range s.requests {
		if (req.RuntimeID == id || req.DaemonID == id) && req.Status == UpdatePending {
			req.Status = UpdateRunning
			req.UpdatedAt = time.Now()
			return req
		}
	}
	return nil
}

// PopAllPending returns and marks as running all pending updates for a daemon or runtime.
func (s *UpdateStore) PopAllPending(id string) []*UpdateRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []*UpdateRequest
	for _, req := range s.requests {
		if (req.DaemonID == id || req.RuntimeID == id) && req.Status == UpdatePending {
			req.Status = UpdateRunning
			req.UpdatedAt = time.Now()
			result = append(result, req)
		}
	}
	return result
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

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// InitiateUpdate creates a new CLI update request (protected route, called by frontend).
func (h *Handler) InitiateUpdate(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireRuntimeAccess(w, r)
	if !ok {
		return
	}

	var req struct {
		TargetVersion string `json:"target_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TargetVersion == "" {
		writeError(w, http.StatusBadRequest, "target_version is required")
		return
	}

	update, err := h.UpdateStore.Create(uuidToString(rt.ID), req.TargetVersion)
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
// It also updates the daemon/runtime DB status to reflect the update lifecycle.
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

	update := h.UpdateStore.Get(updateID)

	switch req.Status {
	case "completed":
		h.UpdateStore.Complete(updateID, req.Output)
	case "failed":
		h.UpdateStore.Fail(updateID, req.Error)
	case "running":
		// Mark daemon/runtime as "updating" in DB.
		if update != nil {
			if update.DaemonID != "" {
				if update.Target == "multica" {
					h.Queries.SetDaemonAndRuntimesUpdating(r.Context(), parseUUID(update.DaemonID))
				} else {
					h.Queries.SetDaemonStatus(r.Context(), db.SetDaemonStatusParams{ID: parseUUID(update.DaemonID), Status: "updating"})
				}
			} else if update.RuntimeID != "" {
				h.Queries.SetRuntimeStatus(r.Context(), db.SetRuntimeStatusParams{ID: parseUUID(update.RuntimeID), Status: "updating"})
			}
		}
	default:
		writeError(w, http.StatusBadRequest, "invalid status: "+req.Status)
		return
	}

	// Restore status to "online" when update finishes (success or failure).
	if (req.Status == "completed" || req.Status == "failed") && update != nil {
		if update.DaemonID != "" {
			h.Queries.SetDaemonStatus(r.Context(), db.SetDaemonStatusParams{ID: parseUUID(update.DaemonID), Status: "online"})
			// For multica updates, also restore runtimes.
			if update.Target == "multica" {
				h.Queries.UpdateRuntimesHeartbeatByDaemon(r.Context(), parseUUID(update.DaemonID))
			}
		} else if update.RuntimeID != "" {
			h.Queries.SetRuntimeStatus(r.Context(), db.SetRuntimeStatusParams{ID: parseUUID(update.RuntimeID), Status: "online"})
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
