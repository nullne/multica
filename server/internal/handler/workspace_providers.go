package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/nullne/multica/server/internal/logger"
	"github.com/nullne/multica/server/pkg/protocol"
)

// ---------------------------------------------------------------------------
// Provider Configuration (workspace-level)
// ---------------------------------------------------------------------------

// ProviderConfig holds the configuration for a single code agent provider.
type ProviderConfig struct {
	Enabled       bool   `json:"enabled"`
	APIKey        string `json:"api_key,omitempty"`
	TargetVersion string `json:"target_version,omitempty"`
}

// WorkspaceProviderSettings is the providers section of workspace.settings.
type WorkspaceProviderSettings struct {
	Providers            map[string]ProviderConfig `json:"providers,omitempty"`
	MulticaTargetVersion string                    `json:"multica_target_version,omitempty"`
}

// SupportedProviders lists the provider keys recognised by multica.
var SupportedProviders = []string{"claude", "codex", "opencode", "cursor"}

// redactAPIKey masks an API key, showing only the last 4 characters.
func redactAPIKey(key string) string {
	if len(key) <= 4 {
		if key == "" {
			return ""
		}
		return "****"
	}
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}

// parseProviderSettings extracts the provider settings from the raw workspace
// settings JSONB. Returns zero-value struct when nothing is configured.
func parseProviderSettings(raw []byte) WorkspaceProviderSettings {
	if raw == nil {
		return WorkspaceProviderSettings{}
	}
	var full map[string]json.RawMessage
	if err := json.Unmarshal(raw, &full); err != nil {
		return WorkspaceProviderSettings{}
	}
	var ps WorkspaceProviderSettings
	if providersRaw, ok := full["providers"]; ok {
		json.Unmarshal(providersRaw, &ps.Providers)
	}
	if versionRaw, ok := full["multica_target_version"]; ok {
		json.Unmarshal(versionRaw, &ps.MulticaTargetVersion)
	}
	return ps
}

// mergeProviderSettingsIntoWorkspace merges the provider settings back into the
// raw workspace settings JSONB, preserving all other fields.
func mergeProviderSettingsIntoWorkspace(raw []byte, ps WorkspaceProviderSettings) ([]byte, error) {
	var full map[string]json.RawMessage
	if raw != nil {
		if err := json.Unmarshal(raw, &full); err != nil {
			full = make(map[string]json.RawMessage)
		}
	}
	if full == nil {
		full = make(map[string]json.RawMessage)
	}

	if ps.Providers != nil {
		b, err := json.Marshal(ps.Providers)
		if err != nil {
			return nil, err
		}
		full["providers"] = b
	}
	if ps.MulticaTargetVersion != "" {
		b, err := json.Marshal(ps.MulticaTargetVersion)
		if err != nil {
			return nil, err
		}
		full["multica_target_version"] = b
	}
	return json.Marshal(full)
}

// redactProviderSettings returns a copy with API keys masked for safe API responses.
func redactProviderSettings(ps WorkspaceProviderSettings) WorkspaceProviderSettings {
	out := WorkspaceProviderSettings{
		MulticaTargetVersion: ps.MulticaTargetVersion,
	}
	if ps.Providers != nil {
		out.Providers = make(map[string]ProviderConfig, len(ps.Providers))
		for k, v := range ps.Providers {
			out.Providers[k] = ProviderConfig{
				Enabled:       v.Enabled,
				APIKey:        redactAPIKey(v.APIKey),
				TargetVersion: v.TargetVersion,
			}
		}
	}
	return out
}

// GetWorkspaceProviders returns the provider configuration for a workspace.
// API keys are redacted in the response.
func (h *Handler) GetWorkspaceProviders(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")

	ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	ps := parseProviderSettings(ws.Settings)
	writeJSON(w, http.StatusOK, redactProviderSettings(ps))
}

// UpdateWorkspaceProviders updates the provider configuration for a workspace.
func (h *Handler) UpdateWorkspaceProviders(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")

	var req WorkspaceProviderSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate provider keys.
	if req.Providers != nil {
		valid := make(map[string]bool, len(SupportedProviders))
		for _, p := range SupportedProviders {
			valid[p] = true
		}
		for k := range req.Providers {
			if !valid[k] {
				writeError(w, http.StatusBadRequest, "unsupported provider: "+k)
				return
			}
		}
	}

	ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	// Merge: if an API key field is all asterisks (redacted), keep the existing value.
	existing := parseProviderSettings(ws.Settings)
	if req.Providers != nil && existing.Providers != nil {
		for k, v := range req.Providers {
			if isRedacted(v.APIKey) {
				// Preserve existing API key when the client sends back the masked value.
				if old, ok := existing.Providers[k]; ok {
					v.APIKey = old.APIKey
					req.Providers[k] = v
				}
			}
		}
	}

	merged, err := mergeProviderSettingsIntoWorkspace(ws.Settings, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode settings")
		return
	}

	ws, err = h.Queries.UpdateWorkspace(r.Context(), updateSettingsOnly(ws.ID, merged))
	if err != nil {
		slog.Warn("update workspace providers failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", id)...)
		writeError(w, http.StatusInternalServerError, "failed to update providers")
		return
	}

	slog.Info("workspace providers updated", append(logger.RequestAttrs(r), "workspace_id", id)...)
	userID := requestUserID(r)
	h.publish(protocol.EventWorkspaceUpdated, id, "member", userID, map[string]any{"workspace": workspaceToResponse(ws)})

	writeJSON(w, http.StatusOK, redactProviderSettings(parseProviderSettings(ws.Settings)))
}

// isRedacted returns true if the string looks like a redacted API key (all * or *…*XXXX).
func isRedacted(s string) bool {
	if s == "" {
		return false
	}
	// A redacted key starts with at least one '*'.
	if s[0] != '*' {
		return false
	}
	// All-asterisks or asterisks followed by up to 4 visible chars (from redactAPIKey).
	starCount := 0
	for _, c := range s {
		if c == '*' {
			starCount++
		}
	}
	return starCount > 0 && len(s)-starCount <= 4
}

// UpdateAllDaemons triggers a version update for all online daemons in a workspace.
func (h *Handler) UpdateAllDaemons(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")

	var req struct {
		Targets []struct {
			Target  string `json:"target"`  // "multica", "claude", "codex", etc.
			Version string `json:"version"` // target version
		} `json:"targets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Targets) == 0 {
		writeError(w, http.StatusBadRequest, "at least one target is required")
		return
	}

	// Get all runtimes for this workspace.
	runtimes, err := h.Queries.ListAgentRuntimes(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runtimes")
		return
	}

	// Find unique online daemon_ids — we only need to update each daemon once.
	// Pick one runtime per daemon to send the update through.
	daemonRuntimes := make(map[string]string) // daemon_id -> runtime_id (first found)
	for _, rt := range runtimes {
		if rt.Status != "online" {
			continue
		}
		daemonID := ""
		if rt.DaemonID.Valid {
			daemonID = rt.DaemonID.String
		}
		if daemonID == "" {
			continue
		}
		if _, exists := daemonRuntimes[daemonID]; !exists {
			daemonRuntimes[daemonID] = uuidToString(rt.ID)
		}
	}

	if len(daemonRuntimes) == 0 {
		writeError(w, http.StatusBadRequest, "no online daemons found")
		return
	}

	// Queue updates for each daemon.
	var queued int
	for _, runtimeID := range daemonRuntimes {
		for _, target := range req.Targets {
			update, err := h.UpdateStore.CreateWithTarget(runtimeID, target.Target, target.Version)
			if err != nil {
				slog.Debug("skip update", "runtime_id", runtimeID, "target", target.Target, "error", err)
				continue
			}
			_ = update
			queued++
		}
	}

	slog.Info("update all daemons", "workspace_id", id, "daemons", len(daemonRuntimes), "queued", queued)
	writeJSON(w, http.StatusOK, map[string]any{
		"daemons_count": len(daemonRuntimes),
		"updates_queued": queued,
	})
}
