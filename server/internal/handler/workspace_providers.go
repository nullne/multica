package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nullne/multica/server/internal/codeagent"
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

type ProviderValidationStatus string

const (
	ProviderValidationValid       ProviderValidationStatus = "valid"
	ProviderValidationInvalid     ProviderValidationStatus = "invalid"
	ProviderValidationUnsupported ProviderValidationStatus = "unsupported"
	ProviderValidationUnavailable ProviderValidationStatus = "temporarily_unavailable"
)

type ProviderValidationResult struct {
	Provider string                   `json:"provider"`
	Status   ProviderValidationStatus `json:"status"`
	Message  string                   `json:"message"`
}

var providerValidationHTTPClient = &http.Client{Timeout: 10 * time.Second}

var providerValidationURLs = map[string]string{
	"claude":   "https://api.anthropic.com/v1/models",
	"codex":    "https://api.openai.com/v1/models",
	"opencode": "https://api.openai.com/v1/models",
}

// SupportedProviders lists the provider keys recognised by multica.
var SupportedProviders = []string{"claude", "codex", "opencode", "cursor"}

// defaultProviderSettings returns the JSON bytes for the default workspace
// provider settings. Target versions are repository-owned and are not stored.
func defaultProviderSettings() []byte {
	b, _ := json.Marshal(WorkspaceProviderSettings{
		Providers: map[string]ProviderConfig{
			"codex": {Enabled: true},
		},
	})
	return b
}

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
	delete(full, "multica_target_version")
	return json.Marshal(full)
}

// redactProviderSettings returns a copy with API keys masked for safe API responses.
func redactProviderSettings(ps WorkspaceProviderSettings) WorkspaceProviderSettings {
	out := WorkspaceProviderSettings{
		Providers: make(map[string]ProviderConfig, len(SupportedProviders)),
	}
	for _, provider := range SupportedProviders {
		cfg := ProviderConfig{}
		if ps.Providers != nil {
			cfg = ps.Providers[provider]
		}
		cfg.APIKey = redactAPIKey(cfg.APIKey)
		cfg.TargetVersion = codeagent.MustVersion(provider)
		out.Providers[provider] = cfg
	}
	return out
}

func rejectConfiguredTargetVersions(ps WorkspaceProviderSettings) error {
	if ps.MulticaTargetVersion != "" {
		return fmt.Errorf("multica_target_version is repository-owned")
	}
	for provider, cfg := range ps.Providers {
		if cfg.TargetVersion != "" {
			return fmt.Errorf("provider %s target_version is repository-owned", provider)
		}
	}
	return nil
}

func daemonProviderConfig(ps WorkspaceProviderSettings) map[string]map[string]any {
	out := make(map[string]map[string]any)
	for provider, cfg := range ps.Providers {
		out[provider] = map[string]any{
			"enabled":        cfg.Enabled,
			"api_key":        cfg.APIKey,
			"target_version": codeagent.MustVersion(provider),
		}
	}
	return out
}

func isSupportedProvider(provider string) bool {
	for _, p := range SupportedProviders {
		if p == provider {
			return true
		}
	}
	return false
}

func validateProviderAPIKey(ctx context.Context, provider, apiKey string) ProviderValidationResult {
	if _, ok := providerValidationURLs[provider]; !ok {
		return ProviderValidationResult{
			Provider: provider,
			Status:   ProviderValidationUnsupported,
			Message:  "Validation is not supported for this provider yet.",
		}
	}
	if strings.TrimSpace(apiKey) == "" {
		return ProviderValidationResult{
			Provider: provider,
			Status:   ProviderValidationInvalid,
			Message:  "No API key is configured for this provider.",
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, providerValidationURLs[provider], nil)
	if err != nil {
		return ProviderValidationResult{
			Provider: provider,
			Status:   ProviderValidationUnavailable,
			Message:  "Could not build provider validation request.",
		}
	}
	switch provider {
	case "claude":
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := providerValidationHTTPClient.Do(req)
	if err != nil {
		return ProviderValidationResult{
			Provider: provider,
			Status:   ProviderValidationUnavailable,
			Message:  "Provider validation is temporarily unavailable.",
		}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return ProviderValidationResult{
			Provider: provider,
			Status:   ProviderValidationValid,
			Message:  "API key is valid.",
		}
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ProviderValidationResult{
			Provider: provider,
			Status:   ProviderValidationInvalid,
			Message:  "Provider rejected the API key.",
		}
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return ProviderValidationResult{
			Provider: provider,
			Status:   ProviderValidationUnavailable,
			Message:  "Provider validation is temporarily unavailable.",
		}
	default:
		return ProviderValidationResult{
			Provider: provider,
			Status:   ProviderValidationInvalid,
			Message:  "Provider rejected the API key.",
		}
	}
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
	if err := rejectConfiguredTargetVersions(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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

// ValidateWorkspaceProvider validates a provider API key without returning or logging it.
func (h *Handler) ValidateWorkspaceProvider(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")
	provider := chi.URLParam(r, "provider")
	if !isSupportedProvider(provider) {
		writeError(w, http.StatusBadRequest, "unsupported provider: "+provider)
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	apiKey := req.APIKey
	if apiKey == "" || isRedacted(apiKey) {
		ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(id))
		if err != nil {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		ps := parseProviderSettings(ws.Settings)
		if cfg, ok := ps.Providers[provider]; ok {
			apiKey = cfg.APIKey
		}
	}

	writeJSON(w, http.StatusOK, validateProviderAPIKey(r.Context(), provider, apiKey))
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

	daemons, err := h.Queries.ListOnlineDaemonsForWorkspace(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list daemons")
		return
	}
	if len(daemons) == 0 {
		writeError(w, http.StatusBadRequest, "no online daemons found")
		return
	}

	var queued int
	for _, d := range daemons {
		daemonUUID := uuidToString(d.ID)
		for _, target := range req.Targets {
			if verified, ok := codeagent.Version(target.Target); ok && target.Version != verified {
				writeError(w, http.StatusBadRequest, "target_version is repository-owned")
				return
			}
			if _, err := h.UpdateStore.CreateForDaemon(daemonUUID, target.Target, target.Version); err != nil {
				slog.Debug("skip update", "daemon_id", daemonUUID, "target", target.Target, "error", err)
				continue
			}
			queued++
		}
	}

	slog.Info("update all daemons", "workspace_id", id, "daemons", len(daemons), "queued", queued)
	writeJSON(w, http.StatusOK, map[string]any{
		"daemons_count":  len(daemons),
		"updates_queued": queued,
	})
}
