package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

// ReceiveGitHubEvent is the public ingest endpoint for GitHub App webhooks.
// Authentication is via HMAC signature (X-Hub-Signature-256). Once the
// signature passes, the request is forwarded to the unified webhook pipeline
// by looking up the matching webhook record via installation_id.
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

	// ping is GitHub's connectivity check — short-circuit before doing any
	// database work since the payload may not include an installation.
	if eventType == "ping" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pong"})
		return
	}

	var envelope struct {
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

	webhook, err := h.Queries.GetWebhookByInstallationID(r.Context(), pgtype.Int8{Int64: envelope.Installation.ID, Valid: true})
	if err != nil {
		// No webhook is registered for this installation. We deliberately
		// return 200 so GitHub does not retry: the workspace either hasn't
		// completed the connect flow yet, or has disconnected the App.
		slog.Debug("github event: no webhook for installation", "installation_id", envelope.Installation.ID)
		w.WriteHeader(http.StatusOK)
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
