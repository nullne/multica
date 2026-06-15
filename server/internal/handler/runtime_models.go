package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// ---------------------------------------------------------------------------
// Model list request store
// ---------------------------------------------------------------------------
//
// The server cannot call the daemon directly (the daemon is behind the user's
// NAT and only polls the server). So "list models for this runtime" uses a
// pending-request pattern: a frontend POST creates a pending request, the
// daemon pops it on the next heartbeat, executes locally, and reports the
// result back. Same shape as PingStore / UpdateStore.

// ModelListStatus represents the lifecycle of a model list request.
type ModelListStatus string

const (
	ModelListPending   ModelListStatus = "pending"
	ModelListRunning   ModelListStatus = "running"
	ModelListCompleted ModelListStatus = "completed"
	ModelListFailed    ModelListStatus = "failed"
	ModelListTimeout   ModelListStatus = "timeout"
)

// ModelListRequest represents a pending or completed model list request.
// Supported is false when the provider ignores per-agent model selection
// entirely; the UI uses this to disable its dropdown rather than silently
// accepting a value the backend will drop.
//
// runStartedAt is server-side bookkeeping for the running timeout; the UI
// only needs Status / UpdatedAt to drive the polling loop.
type ModelListRequest struct {
	ID           string          `json:"id"`
	RuntimeID    string          `json:"runtime_id"`
	Status       ModelListStatus `json:"status"`
	Models       []ModelEntry    `json:"models,omitempty"`
	Supported    bool            `json:"supported"`
	Error        string          `json:"error,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	runStartedAt *time.Time
}

// ModelEntry mirrors agent.Model for the wire. `Default` tags the model the
// runtime advertises as its preferred pick so the UI can badge it.
//
// `Thinking` carries the per-model reasoning-effort catalog discovered by
// the daemon for runtimes that support it (claude, codex, opencode). nil
// means "no picker for this model"; the UI hides the thinking_level selector.
type ModelEntry struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Provider string         `json:"provider,omitempty"`
	Default  bool           `json:"default,omitempty"`
	Thinking *ModelThinking `json:"thinking,omitempty"`
}

// ModelThinking is the wire shape for the per-model thinking catalog.
// Mirrors agent.ModelThinking so the daemon's report passes through
// without remapping.
type ModelThinking struct {
	SupportedLevels []ThinkingLevelEntry `json:"supported_levels"`
	DefaultLevel    string               `json:"default_level,omitempty"`
}

// ThinkingLevelEntry is the wire shape for a single entry in a model's
// reasoning-effort catalog. `Value` is the literal token the daemon passes
// to the CLI; `Label` is the display string; `Description` is optional
// helper copy (Codex's debug-models output includes one per level).
type ThinkingLevelEntry struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

const (
	// modelListPendingTimeout bounds how long a pending request can sit in
	// the store before the UI is told "daemon didn't pick this up".
	modelListPendingTimeout = 30 * time.Second
	// modelListRunningTimeout bounds how long a claimed (running) request
	// can stay claimed before the UI is told "daemon picked this up but
	// never reported a result" (e.g. the heartbeat response carrying the
	// pending entry was lost in transit after PopPending mutated state).
	modelListRunningTimeout = 60 * time.Second
	// modelListStoreRetention bounds how long any stored request lives.
	// Wider than the running/pending timeouts so terminal records are
	// still readable when the UI's last poll arrives.
	modelListStoreRetention = 2 * time.Minute
)

// applyModelListTimeout transitions a request to ModelListTimeout when it has
// been stuck in a non-terminal state past its threshold. Returns true when
// the record was modified.
func applyModelListTimeout(req *ModelListRequest, now time.Time) bool {
	switch req.Status {
	case ModelListPending:
		if now.Sub(req.CreatedAt) > modelListPendingTimeout {
			req.Status = ModelListTimeout
			req.Error = "daemon did not respond within 30 seconds"
			req.UpdatedAt = now
			return true
		}
	case ModelListRunning:
		if req.runStartedAt != nil && now.Sub(*req.runStartedAt) > modelListRunningTimeout {
			req.Status = ModelListTimeout
			req.Error = "daemon did not finish within 60 seconds"
			req.UpdatedAt = now
			return true
		}
	}
	return false
}

// ModelListStore is a thread-safe in-memory store for model list requests.
type ModelListStore struct {
	mu       sync.Mutex
	requests map[string]*ModelListRequest // keyed by request ID
}

func NewModelListStore() *ModelListStore {
	return &ModelListStore{requests: make(map[string]*ModelListRequest)}
}

func (s *ModelListStore) Create(runtimeID string) *ModelListRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Garbage-collect stale entries so the map can't grow unbounded.
	for id, req := range s.requests {
		if time.Since(req.CreatedAt) > modelListStoreRetention {
			delete(s.requests, id)
		}
	}

	now := time.Now()
	req := &ModelListRequest{
		ID:        randomID(),
		RuntimeID: runtimeID,
		Status:    ModelListPending,
		// Default to true; the daemon overrides this in the report for
		// providers that don't support per-agent model selection.
		Supported: true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.requests[req.ID] = req
	return req
}

func (s *ModelListStore) Get(id string) *ModelListRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[id]
	if !ok {
		return nil
	}
	applyModelListTimeout(req, time.Now())
	return req
}

// PopPending claims the oldest pending request for a runtime (moving it to
// running) or returns nil when there is none.
func (s *ModelListStore) PopPending(runtimeID string) *ModelListRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	var oldest *ModelListRequest
	now := time.Now()
	for _, req := range s.requests {
		applyModelListTimeout(req, now)
		if req.RuntimeID == runtimeID && req.Status == ModelListPending {
			if oldest == nil || req.CreatedAt.Before(oldest.CreatedAt) {
				oldest = req
			}
		}
	}
	if oldest != nil {
		oldest.Status = ModelListRunning
		startedAt := now
		oldest.runStartedAt = &startedAt
		oldest.UpdatedAt = now
	}
	return oldest
}

// PopPendingForRuntimes pops at most one pending request per runtime ID.
// Used by the daemon heartbeat, which is daemon-scoped while requests are
// runtime-scoped.
func (s *ModelListStore) PopPendingForRuntimes(runtimeIDs []string) []*ModelListRequest {
	var out []*ModelListRequest
	for _, id := range runtimeIDs {
		if req := s.PopPending(id); req != nil {
			out = append(out, req)
		}
	}
	return out
}

func (s *ModelListStore) Complete(id string, models []ModelEntry, supported bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req, ok := s.requests[id]; ok {
		req.Status = ModelListCompleted
		req.Models = models
		req.Supported = supported
		req.UpdatedAt = time.Now()
	}
}

func (s *ModelListStore) Fail(id string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req, ok := s.requests[id]; ok {
		req.Status = ModelListFailed
		req.Error = errMsg
		req.UpdatedAt = time.Now()
	}
}

func modelListRequestTerminal(status ModelListStatus) bool {
	return status == ModelListCompleted || status == ModelListFailed || status == ModelListTimeout
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// InitiateListModels creates a pending model list request for a runtime.
// Called by the frontend; the daemon picks it up on its next heartbeat.
func (h *Handler) InitiateListModels(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireRuntimeAccess(w, r)
	if !ok {
		return
	}
	if rt.Status != "online" {
		writeError(w, http.StatusServiceUnavailable, "runtime is offline")
		return
	}

	req := h.ModelListStore.Create(uuidToString(rt.ID))
	writeJSON(w, http.StatusOK, req)
}

// GetModelListRequest returns the status of a model list request.
func (h *Handler) GetModelListRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "requestId")

	req := h.ModelListStore.Get(requestID)
	if req == nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	writeJSON(w, http.StatusOK, req)
}

// ReportModelListResult receives the list result from the daemon.
func (h *Handler) ReportModelListResult(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	requestID := chi.URLParam(r, "requestId")

	// Fetch first so we can ignore stale reports for already-terminal
	// requests (e.g. the heartbeat response that triggered the daemon
	// run was a retry, and the original report already landed).
	existing := h.ModelListStore.Get(requestID)
	if existing == nil || existing.RuntimeID != runtimeID {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if modelListRequestTerminal(existing.Status) {
		slog.Debug("ignoring stale model list report", "runtime_id", runtimeID, "request_id", requestID, "status", existing.Status)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	var body struct {
		Status    string       `json:"status"` // "completed" or "failed"
		Models    []ModelEntry `json:"models"`
		Supported *bool        `json:"supported"`
		Error     string       `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Status == "completed" {
		supported := true
		if body.Supported != nil {
			supported = *body.Supported
		}
		h.ModelListStore.Complete(requestID, body.Models, supported)
	} else {
		h.ModelListStore.Fail(requestID, body.Error)
	}

	slog.Debug("model list report", "runtime_id", runtimeID, "request_id", requestID, "status", body.Status, "count", len(body.Models))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
