package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nullne/multica/server/internal/cli"
	"github.com/nullne/multica/server/internal/daemon/execenv"
	"github.com/nullne/multica/server/internal/daemon/repocache"
	"github.com/nullne/multica/server/internal/daemon/usage"
	"github.com/nullne/multica/server/pkg/agent"
)

// workspaceState tracks the runtimes the server projected for one workspace
// (per-(workspace, provider) rows the scheduler dispatches to).
type workspaceState struct {
	workspaceID string
	enabled     bool
	runtimeIDs  []string
}

// Daemon is the local agent runtime that polls for and executes tasks.
type Daemon struct {
	cfg       Config
	client    *Client
	repoCache *repocache.Cache
	logger    *slog.Logger

	mu              sync.Mutex
	daemonUUID      string // server-assigned daemon row UUID (user-scoped)
	workspaces      map[string]*workspaceState
	runtimeIndex    map[string]Runtime        // runtimeID -> Runtime for provider lookups
	taskBranches    map[string]string         // taskID -> latest checked out branch
	taskTokens      map[string]string         // taskID -> GitHub installation token (cleared on task end)
	providerConfigs map[string]ProviderConfig // provider -> workspace-level config (API keys, etc.)
	reloading       sync.Mutex                // prevents concurrent reload

	cancelFunc    context.CancelFunc // set by Run(); called by triggerRestart
	restartBinary string             // non-empty after a successful update; path to the new binary
	updating      atomic.Bool        // prevents concurrent update attempts
	reconciling   atomic.Bool        // prevents concurrent reconcileProviders
}

// New creates a new Daemon instance.
func New(cfg Config, logger *slog.Logger) *Daemon {
	cacheRoot := filepath.Join(cfg.WorkspacesRoot, ".repos")
	return &Daemon{
		cfg:             cfg,
		client:          NewClient(cfg.ServerBaseURL),
		repoCache:       repocache.New(cacheRoot, logger),
		logger:          logger,
		workspaces:      make(map[string]*workspaceState),
		runtimeIndex:    make(map[string]Runtime),
		taskBranches:    make(map[string]string),
		taskTokens:      make(map[string]string),
		providerConfigs: make(map[string]ProviderConfig),
	}
}

// Run starts the daemon: resolves auth, registers runtimes, then polls for tasks.
func (d *Daemon) Run(ctx context.Context) error {
	// Wrap context so handleUpdate can cancel the daemon for restart.
	ctx, cancel := context.WithCancel(ctx)
	d.cancelFunc = cancel

	// Bind health port early to detect another running daemon.
	healthLn, err := d.listenHealth()
	if err != nil {
		return err
	}

	agentNames := make([]string, 0, len(d.cfg.Agents))
	for name := range d.cfg.Agents {
		agentNames = append(agentNames, name)
	}
	logFields := []any{"version", d.cfg.CLIVersion, "agents", agentNames, "server", d.cfg.ServerBaseURL}
	if d.cfg.Profile != "" {
		logFields = append(logFields, "profile", d.cfg.Profile)
	}
	d.logger.Info("starting daemon", logFields...)

	// Load auth token from CLI config.
	if err := d.resolveAuth(); err != nil {
		return err
	}

	// Register once with the server (user-scoped) and load workspace
	// assignments from the response.
	if err := d.registerWithServer(ctx); err != nil {
		return err
	}

	// Deregister runtimes on shutdown (uses a fresh context since ctx will be cancelled).
	defer d.deregisterDaemon()

	if len(d.allRuntimeIDs()) == 0 {
		d.logger.Warn("no runtimes registered — daemon will wait for providers to become available via heartbeat")
	}

	go d.heartbeatLoop(ctx)
	go d.assignmentSyncLoop(ctx)
	go d.usageScanLoop(ctx)
	go d.serveHealth(ctx, healthLn, time.Now())
	return d.pollLoop(ctx)
}

// RestartBinary returns the path to the new binary if the daemon needs to restart
// after a successful update, or empty string if no restart is needed.
func (d *Daemon) RestartBinary() string {
	return d.restartBinary
}

// deregisterDaemon notifies the server that the daemon is shutting down so
// runtimes can be flipped to offline immediately rather than waiting for the
// stale sweeper.
func (d *Daemon) deregisterDaemon() {
	d.mu.Lock()
	uuid := d.daemonUUID
	d.mu.Unlock()
	if uuid == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.client.DeregisterDaemon(ctx, uuid); err != nil {
		d.logger.Warn("failed to deregister daemon on shutdown", "error", err)
		return
	}
	d.logger.Info("deregistered daemon")
}

// resolveAuth loads the auth token from the CLI config for the active profile.
func (d *Daemon) resolveAuth() error {
	cfg, err := cli.LoadCLIConfigForProfile(d.cfg.Profile)
	if err != nil {
		return fmt.Errorf("load CLI config: %w", err)
	}
	if cfg.Token == "" {
		loginHint := "'multica login'"
		if d.cfg.Profile != "" {
			loginHint = fmt.Sprintf("'multica login --profile %s'", d.cfg.Profile)
		}
		d.logger.Warn("not authenticated — run " + loginHint + " to authenticate, then restart the daemon")
		return fmt.Errorf("not authenticated: run %s first", loginHint)
	}
	d.client.SetToken(cfg.Token)
	d.logger.Info("authenticated")
	return nil
}

// registerWithServer registers this daemon (user-scoped) and projects the
// returned workspace assignments into local state. The daemon now learns the
// set of workspaces it serves from the server rather than from a local
// watched-workspaces config — workspace assignments are managed via the API
// (see `multica daemon enable <workspace-id>`).
func (d *Daemon) registerWithServer(ctx context.Context) error {
	runtimes := d.detectLocalRuntimes(ctx)

	// Bootstrap path: nothing installed locally yet. Register with no
	// runtimes so the server still returns provider_config; auto-install
	// missing CLIs; re-detect and register again.
	if len(runtimes) == 0 {
		boot, err := d.doRegister(ctx, nil)
		if err != nil {
			return fmt.Errorf("bootstrap register: %w", err)
		}
		if resp := boot; resp.Daemon != nil {
			d.mu.Lock()
			d.daemonUUID = resp.Daemon.ID
			d.mu.Unlock()
		}
		if boot.ProviderConfig != nil {
			d.mu.Lock()
			for k, v := range boot.ProviderConfig {
				d.providerConfigs[k] = v
			}
			d.mu.Unlock()
			d.autoInstallProviders(ctx, boot.ProviderConfig)
		}
		runtimes = d.detectLocalRuntimes(ctx)
	}

	resp, err := d.doRegister(ctx, runtimes)
	if err != nil {
		return fmt.Errorf("register daemon: %w", err)
	}

	if resp.Daemon != nil {
		d.mu.Lock()
		d.daemonUUID = resp.Daemon.ID
		d.mu.Unlock()
	}

	if resp.ProviderConfig != nil {
		d.mu.Lock()
		for k, v := range resp.ProviderConfig {
			d.providerConfigs[k] = v
		}
		d.mu.Unlock()

		agentsBefore := len(d.cfg.Agents)
		d.autoInstallProviders(ctx, resp.ProviderConfig)
		if len(d.cfg.Agents) > agentsBefore {
			updated := d.detectLocalRuntimes(ctx)
			if len(updated) > len(runtimes) {
				d.logger.Info("re-registering with newly installed providers", "before", len(runtimes), "after", len(updated))
				if reResp, err := d.doRegister(ctx, updated); err == nil {
					resp = reResp
				}
			}
		}
	}

	d.applyRegisterResponse(resp)

	activeCount := countEnabledWorkspaces(resp.Workspaces)
	switch {
	case activeCount == 0:
		d.logger.Warn("daemon registered but has no workspace assignments — use 'multica daemon enable <workspace-id>' to enable one")
	default:
		for _, ws := range resp.Workspaces {
			if !ws.Enabled {
				continue
			}
			d.logger.Info("workspace assignment active",
				"workspace_id", ws.WorkspaceID,
				"name", ws.WorkspaceName,
				"runtimes", len(ws.Runtimes),
			)
		}
	}
	return nil
}

// applyRegisterResponse replaces the daemon's workspace+runtime state with the
// projection returned by the server.
func (d *Daemon) applyRegisterResponse(resp *RegisterResponse) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.workspaces = make(map[string]*workspaceState, len(resp.Workspaces))
	d.runtimeIndex = make(map[string]Runtime)
	for _, ws := range resp.Workspaces {
		runtimeIDs := make([]string, len(ws.Runtimes))
		for i, rt := range ws.Runtimes {
			runtimeIDs[i] = rt.ID
			d.runtimeIndex[rt.ID] = rt
		}
		d.workspaces[ws.WorkspaceID] = &workspaceState{
			workspaceID: ws.WorkspaceID,
			enabled:     ws.Enabled,
			runtimeIDs:  runtimeIDs,
		}
	}
}

func countEnabledWorkspaces(workspaces []WorkspaceRegistration) int {
	count := 0
	for _, ws := range workspaces {
		if ws.Enabled {
			count++
		}
	}
	return count
}

// allRuntimeIDs returns all runtime IDs across all projected workspaces,
// including hidden ones kept alive for explicit daemon-targeted dispatch.
func (d *Daemon) allRuntimeIDs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var ids []string
	for _, ws := range d.workspaces {
		ids = append(ids, ws.runtimeIDs...)
	}
	return ids
}

// findRuntime looks up a Runtime by its ID.
func (d *Daemon) findRuntime(id string) *Runtime {
	d.mu.Lock()
	defer d.mu.Unlock()
	if rt, ok := d.runtimeIndex[id]; ok {
		return &rt
	}
	return nil
}

// providerToRuntimeMap returns a mapping from provider name to runtime ID.
func (d *Daemon) providerToRuntimeMap() map[string]string {
	d.mu.Lock()
	defer d.mu.Unlock()
	m := make(map[string]string)
	for id, rt := range d.runtimeIndex {
		m[rt.Provider] = id
	}
	return m
}

// detectLocalRuntimes probes locally installed agent CLIs and returns their
// registration payloads.
func (d *Daemon) detectLocalRuntimes(ctx context.Context) []map[string]string {
	var runtimes []map[string]string
	for name, entry := range d.cfg.Agents {
		version, err := agent.DetectVersion(ctx, entry.Path)
		if err != nil {
			d.logger.Warn("skip registering runtime", "name", name, "error", err)
			continue
		}
		authStatus := agent.CheckAuth(ctx, name, entry.Path)
		displayName := strings.ToUpper(name[:1]) + name[1:]
		if d.cfg.DeviceName != "" {
			displayName = fmt.Sprintf("%s (%s)", displayName, d.cfg.DeviceName)
		}
		runtimes = append(runtimes, map[string]string{
			"name":        displayName,
			"type":        name,
			"version":     version,
			"status":      "online",
			"auth_status": authStatus,
		})
		d.logger.Info("detected provider auth", "provider", name, "auth_status", authStatus)
	}
	return runtimes
}

// doRegister sends a registration request to the server. runtimes may be nil
// for a bootstrap registration (to fetch provider config before any CLIs are installed).
func (d *Daemon) doRegister(ctx context.Context, runtimes []map[string]string) (*RegisterResponse, error) {
	if runtimes == nil {
		runtimes = []map[string]string{}
	}
	req := map[string]any{
		"daemon_id":   d.cfg.DaemonID,
		"device_name": d.cfg.DeviceName,
		"cli_version": d.cfg.CLIVersion,
		"runtimes":    runtimes,
		"env_vars":    collectSanitizedEnv(),
	}
	return d.client.Register(ctx, req)
}

// collectSanitizedEnv returns the current process environment as a key-value
// map with sensitive values masked (keys containing KEY, SECRET, TOKEN, etc.).
func collectSanitizedEnv() map[string]string {
	sensitiveSubstrings := []string{
		"KEY", "SECRET", "TOKEN", "PASSWORD", "PASSWD",
		"CREDENTIAL", "AUTH", "PRIVATE",
	}
	env := make(map[string]string)
	for _, e := range os.Environ() {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(k)
		masked := false
		for _, sub := range sensitiveSubstrings {
			if strings.Contains(upper, sub) {
				env[k] = "********"
				masked = true
				break
			}
		}
		if !masked {
			env[k] = v
		}
	}
	return env
}

// reconcileProviders checks heartbeat provider config against locally installed
// agents and auto-installs + re-registers any missing ones.
func (d *Daemon) reconcileProviders(ctx context.Context, config map[string]ProviderConfig) {
	d.mu.Lock()
	for k, v := range config {
		d.providerConfigs[k] = v
	}
	d.mu.Unlock()

	agentsBefore := len(d.cfg.Agents)
	d.autoInstallProviders(ctx, config)

	if len(d.cfg.Agents) <= agentsBefore {
		return
	}

	// New agents installed — re-register the daemon so the server projects
	// the new runtime rows into every enabled workspace.
	runtimes := d.detectLocalRuntimes(ctx)
	resp, err := d.doRegister(ctx, runtimes)
	if err != nil {
		d.logger.Warn("re-register after auto-install failed", "error", err)
		return
	}
	d.applyRegisterResponse(resp)
	d.logger.Info("re-registered daemon with new providers", "active_workspaces", countEnabledWorkspaces(resp.Workspaces), "projected_workspaces", len(resp.Workspaces))
}

// autoInstallProviders installs code agent CLIs that are enabled in the workspace
// config but not yet available on this machine. After installing, it re-probes
// and adds them to the agent config.
func (d *Daemon) autoInstallProviders(ctx context.Context, providerConfig map[string]ProviderConfig) {
	for provider, cfg := range providerConfig {
		if !cfg.Enabled {
			continue
		}
		// Skip if already available.
		if _, ok := d.cfg.Agents[provider]; ok {
			continue
		}

		d.logger.Info("provider enabled in workspace but not installed locally, auto-installing", "provider", provider)
		_, err := InstallProvider(ctx, provider, cfg.TargetVersion, d.logger)
		if err != nil {
			d.logger.Warn("auto-install failed, provider will not be available", "provider", provider, "error", err)
			continue
		}

		// Re-probe the installed CLI.
		binName := providerBinaryName(provider)
		path, lookupErr := exec.LookPath(binName)
		if lookupErr != nil {
			// Some installers (e.g. cursor) place binaries outside $PATH.
			for _, fallback := range providerFallbackPaths(binName) {
				if _, err := os.Stat(fallback); err == nil {
					path = fallback
					break
				}
			}
			if path == "" {
				d.logger.Warn("auto-install succeeded but CLI not found on PATH", "provider", provider, "error", lookupErr)
				continue
			}
		}

		d.cfg.Agents[provider] = AgentEntry{Path: path}
		d.logger.Info("auto-installed code agent CLI", "provider", provider, "path", path)
	}
}

// resolveProviderAPIKey returns the API key to use for a provider.
// Prefers the task-level key (from task claim), falls back to the cached
// workspace-level key, and returns empty if neither is set (user self-auth).
func (d *Daemon) resolveProviderAPIKey(provider, taskAPIKey string) string {
	if taskAPIKey != "" {
		return taskAPIKey
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if cfg, ok := d.providerConfigs[provider]; ok {
		return cfg.APIKey
	}
	return ""
}

// providerFallbackPaths returns additional paths to check when the binary is
// not on $PATH. Some installers (e.g. cursor) place binaries in ~/.local/bin.
func providerFallbackPaths(binName string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "bin", binName),
		filepath.Join("/usr", "local", "bin", binName),
	}
}

// providerBinaryName returns the expected binary name for a provider.
func providerBinaryName(provider string) string {
	switch provider {
	case "claude":
		return "claude"
	case "codex":
		return "codex"
	case "opencode":
		return "opencode"
	case "cursor":
		return "agent"
	default:
		return provider
	}
}

// assignmentSyncLoop periodically polls the server for the daemon's enabled
// workspace assignments and re-registers when they change. Server-side
// daemon_workspace is the source of truth, so the daemon picks up enable /
// disable operations issued through the API or CLI without needing a restart.
func (d *Daemon) assignmentSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(DefaultWorkspaceSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.syncAssignments(ctx)
		}
	}
}

// syncAssignments compares the server's enabled-workspace set for this daemon
// against local state and re-registers if they diverge.
func (d *Daemon) syncAssignments(ctx context.Context) {
	d.reloading.Lock()
	defer d.reloading.Unlock()

	d.mu.Lock()
	daemonUUID := d.daemonUUID
	currentIDs := make(map[string]bool, len(d.workspaces))
	for id, ws := range d.workspaces {
		if ws.enabled {
			currentIDs[id] = true
		}
	}
	d.mu.Unlock()

	if daemonUUID == "" {
		return
	}

	apiCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	assignments, err := d.client.ListMyDaemonWorkspaces(apiCtx, daemonUUID)
	if err != nil {
		d.logger.Debug("assignment sync failed", "error", err)
		return
	}

	enabledIDs := make(map[string]bool)
	for _, a := range assignments {
		if a.Enabled {
			enabledIDs[a.WorkspaceID] = true
		}
	}

	if mapsEqual(currentIDs, enabledIDs) {
		return
	}

	d.logger.Info("daemon assignments changed, re-registering", "before", len(currentIDs), "after", len(enabledIDs))
	resp, err := d.doRegister(ctx, d.detectLocalRuntimes(ctx))
	if err != nil {
		d.logger.Warn("re-register after assignment change failed", "error", err)
		return
	}
	d.applyRegisterResponse(resp)
}

func mapsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func (d *Daemon) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(d.cfg.HeartbeatInterval)
	defer ticker.Stop()

	authCheckCounter := 0
	authCheckInterval := max(60/int(d.cfg.HeartbeatInterval.Seconds()), 1)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			authCheckCounter++
			refreshAuth := authCheckCounter%authCheckInterval == 0

			var authStatuses map[string]string
			if refreshAuth {
				authStatuses = make(map[string]string)
				for name, entry := range d.cfg.Agents {
					authStatuses[name] = agent.CheckAuth(ctx, name, entry.Path)
				}
			}

			d.mu.Lock()
			daemonUUID := d.daemonUUID
			var firstRuntimeID string
			for _, ws := range d.workspaces {
				if len(ws.runtimeIDs) > 0 {
					firstRuntimeID = ws.runtimeIDs[0]
					break
				}
			}
			d.mu.Unlock()

			if daemonUUID == "" {
				continue
			}

			resp, err := d.client.SendDaemonHeartbeat(ctx, daemonUUID, authStatuses)
			if err != nil {
				d.logger.Warn("heartbeat failed", "daemon_uuid", daemonUUID, "error", err)
				continue
			}

			if resp.PendingPing != nil && firstRuntimeID != "" {
				if rt := d.findRuntime(firstRuntimeID); rt != nil {
					go d.handlePing(ctx, *rt, resp.PendingPing.ID)
				}
			}

			for _, update := range resp.PendingUpdates {
				u := update
				go d.handleUpdate(ctx, daemonUUID, &u)
			}

			// Auto-install newly enabled providers (same cadence as auth check).
			if refreshAuth && len(resp.ProviderConfig) > 0 && d.reconciling.CompareAndSwap(false, true) {
				go func() {
					defer d.reconciling.Store(false)
					d.reconcileProviders(ctx, resp.ProviderConfig)
				}()
			}
		}
	}
}

func (d *Daemon) handlePing(ctx context.Context, rt Runtime, pingID string) {
	d.logger.Info("ping requested", "runtime_id", rt.ID, "ping_id", pingID, "provider", rt.Provider)

	start := time.Now()

	entry, ok := d.cfg.Agents[rt.Provider]
	if !ok {
		d.client.ReportPingResult(ctx, rt.ID, pingID, map[string]any{
			"status":      "failed",
			"error":       fmt.Sprintf("no agent configured for provider %q", rt.Provider),
			"duration_ms": time.Since(start).Milliseconds(),
		})
		return
	}

	backend, err := agent.New(rt.Provider, agent.Config{
		ExecutablePath: entry.Path,
		Logger:         d.logger,
	})
	if err != nil {
		d.client.ReportPingResult(ctx, rt.ID, pingID, map[string]any{
			"status":      "failed",
			"error":       err.Error(),
			"duration_ms": time.Since(start).Milliseconds(),
		})
		return
	}

	pingCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	session, err := backend.Execute(pingCtx, "Respond with exactly one word: pong", agent.ExecOptions{
		MaxTurns: 1,
		Timeout:  60 * time.Second,
	})
	if err != nil {
		d.client.ReportPingResult(ctx, rt.ID, pingID, map[string]any{
			"status":      "failed",
			"error":       err.Error(),
			"duration_ms": time.Since(start).Milliseconds(),
		})
		return
	}

	// Drain messages
	go func() {
		for range session.Messages {
		}
	}()

	result := <-session.Result
	durationMs := time.Since(start).Milliseconds()

	if result.Status == "completed" {
		d.logger.Info("ping completed", "runtime_id", rt.ID, "ping_id", pingID, "duration_ms", durationMs)
		d.client.ReportPingResult(ctx, rt.ID, pingID, map[string]any{
			"status":      "completed",
			"output":      result.Output,
			"duration_ms": durationMs,
		})
	} else {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("agent returned status: %s", result.Status)
		}
		d.logger.Warn("ping failed", "runtime_id", rt.ID, "ping_id", pingID, "error", errMsg)
		d.client.ReportPingResult(ctx, rt.ID, pingID, map[string]any{
			"status":      "failed",
			"error":       errMsg,
			"duration_ms": durationMs,
		})
	}
}

// handleUpdate performs the CLI update when triggered by the server via heartbeat.
// The id parameter can be a daemon UUID or a legacy runtime ID.
func (d *Daemon) handleUpdate(ctx context.Context, id string, update *PendingUpdate) {
	if !d.updating.CompareAndSwap(false, true) {
		d.logger.Warn("update already in progress, ignoring", "id", id, "update_id", update.ID)
		return
	}
	defer d.updating.Store(false)

	target := update.Target
	if target == "" {
		target = "multica"
	}

	// Use first runtime ID for reporting update results (API still requires a runtime path).
	reportRuntimeID := id
	rids := d.allRuntimeIDs()
	if len(rids) > 0 {
		reportRuntimeID = rids[0]
	}

	d.logger.Info("update requested", "id", id, "update_id", update.ID, "target", target, "target_version", update.TargetVersion)

	d.client.ReportUpdateResult(ctx, reportRuntimeID, update.ID, map[string]any{
		"status": "running",
	})

	if target == "multica" {
		d.handleMulticaUpdate(ctx, reportRuntimeID, update)
	} else {
		d.handleProviderUpdate(ctx, reportRuntimeID, update, target)
	}
}

// handleMulticaUpdate handles updates to the multica CLI itself.
func (d *Daemon) handleMulticaUpdate(ctx context.Context, runtimeID string, update *PendingUpdate) {
	// Try Homebrew first, fall back to direct download.
	var output string
	if cli.IsBrewInstall() {
		d.logger.Info("updating CLI via Homebrew...")
		var err error
		output, err = cli.UpdateViaBrew()
		if err != nil {
			d.logger.Error("CLI update failed", "error", err, "output", output)
			d.client.ReportUpdateResult(ctx, runtimeID, update.ID, map[string]any{
				"status": "failed",
				"error":  fmt.Sprintf("brew upgrade failed: %v", err),
			})
			return
		}
	} else {
		d.logger.Info("updating CLI via direct download...", "target_version", update.TargetVersion)
		var err error
		output, err = cli.UpdateViaDownload(update.TargetVersion)
		if err != nil {
			d.logger.Error("CLI update failed", "error", err)
			d.client.ReportUpdateResult(ctx, runtimeID, update.ID, map[string]any{
				"status": "failed",
				"error":  fmt.Sprintf("download update failed: %v", err),
			})
			return
		}
	}

	d.logger.Info("CLI update completed successfully", "output", output)
	d.client.ReportUpdateResult(ctx, runtimeID, update.ID, map[string]any{
		"status": "completed",
		"output": fmt.Sprintf("Updated to %s", update.TargetVersion),
	})

	// Trigger daemon restart with the new binary.
	d.triggerRestart()
}

// handleProviderUpdate handles updates to code agent CLIs (claude, codex, etc.).
func (d *Daemon) handleProviderUpdate(ctx context.Context, runtimeID string, update *PendingUpdate, provider string) {
	output, err := UpdateProvider(ctx, provider, update.TargetVersion, d.logger)
	if err != nil {
		d.client.ReportUpdateResult(ctx, runtimeID, update.ID, map[string]any{
			"status": "failed",
			"error":  err.Error(),
		})
		return
	}

	// Re-probe the updated CLI and update the agent entry.
	path, lookupErr := exec.LookPath(providerBinaryName(provider))
	if lookupErr == nil {
		d.cfg.Agents[provider] = AgentEntry{Path: path}
	}

	d.logger.Info("code agent CLI updated", "provider", provider, "output", output)
	d.client.ReportUpdateResult(ctx, runtimeID, update.ID, map[string]any{
		"status": "completed",
		"output": fmt.Sprintf("Updated %s to %s", provider, update.TargetVersion),
	})

	// No daemon restart needed — just re-register to update version info.
	// The next heartbeat cycle will pick up the new version.
}

// triggerRestart initiates a graceful daemon restart after a successful CLI update.
// For brew installs, it keeps the symlink path (e.g. /opt/homebrew/bin/multica)
// so the restarted daemon picks up the new Cellar version automatically.
// For non-brew installs, it resolves to the absolute path of the replaced binary.
// The caller (cmd_daemon.go) checks RestartBinary() and launches the new process.
func (d *Daemon) triggerRestart() {
	newBin, err := os.Executable()
	if err != nil {
		d.logger.Error("could not resolve executable path for restart", "error", err)
		return
	}
	// Only resolve symlinks for non-brew installs. Brew uses a symlink that
	// points to the latest Cellar version, so we must preserve it.
	if !cli.IsBrewInstall() {
		if resolved, err := filepath.EvalSymlinks(newBin); err == nil {
			newBin = resolved
		}
	}

	d.logger.Info("scheduling daemon restart", "new_binary", newBin)
	d.restartBinary = newBin

	// Cancel the main context to trigger graceful shutdown.
	if d.cancelFunc != nil {
		d.cancelFunc()
	}
}

func (d *Daemon) usageScanLoop(ctx context.Context) {
	scanner := usage.NewScanner(d.logger)

	report := func() {
		records := scanner.Scan()
		if len(records) == 0 {
			return
		}

		// Build provider -> runtime ID mapping from current state.
		providerToRuntime := d.providerToRuntimeMap()

		// Group records by provider to send to the correct runtime.
		byProvider := make(map[string][]map[string]any)
		for _, r := range records {
			byProvider[r.Provider] = append(byProvider[r.Provider], map[string]any{
				"date":               r.Date,
				"provider":           r.Provider,
				"model":              r.Model,
				"input_tokens":       r.InputTokens,
				"output_tokens":      r.OutputTokens,
				"cache_read_tokens":  r.CacheReadTokens,
				"cache_write_tokens": r.CacheWriteTokens,
			})
		}

		for provider, entries := range byProvider {
			runtimeID, ok := providerToRuntime[provider]
			if !ok {
				d.logger.Debug("no runtime for provider, skipping usage report", "provider", provider)
				continue
			}
			if err := d.client.ReportUsage(ctx, runtimeID, entries); err != nil {
				d.logger.Warn("usage report failed", "provider", provider, "runtime_id", runtimeID, "error", err)
			} else {
				d.logger.Info("usage reported", "provider", provider, "runtime_id", runtimeID, "entries", len(entries))
			}
		}
	}

	// Initial scan on startup.
	report()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report()
		}
	}
}

func (d *Daemon) pollLoop(ctx context.Context) error {
	sem := make(chan struct{}, d.cfg.MaxConcurrentTasks)
	var wg sync.WaitGroup

	pollOffset := 0
	pollCount := 0
	for {
		select {
		case <-ctx.Done():
			d.logger.Info("poll loop stopping, waiting for in-flight tasks", "max_wait", "30s")
			waitDone := make(chan struct{})
			go func() { wg.Wait(); close(waitDone) }()
			select {
			case <-waitDone:
			case <-time.After(30 * time.Second):
				d.logger.Warn("timed out waiting for in-flight tasks")
			}
			return ctx.Err()
		default:
		}

		// Skip polling while a CLI update is in progress.
		if d.updating.Load() || d.reconciling.Load() {
			if err := sleepWithContext(ctx, d.cfg.PollInterval); err != nil {
				wg.Wait()
				return err
			}
			continue
		}

		runtimeIDs := d.allRuntimeIDs()
		if len(runtimeIDs) == 0 {
			if err := sleepWithContext(ctx, d.cfg.PollInterval); err != nil {
				wg.Wait()
				return err
			}
			continue
		}

		claimed := false
		n := len(runtimeIDs)
		for i := 0; i < n; i++ {
			// Check if we have capacity before claiming.
			select {
			case sem <- struct{}{}:
				// Acquired a slot.
			default:
				// All slots occupied, stop trying to claim.
				d.logger.Debug("poll: at capacity", "running", d.cfg.MaxConcurrentTasks)
				goto sleep
			}

			rid := runtimeIDs[(pollOffset+i)%n]
			task, err := d.client.ClaimTask(ctx, rid)
			if err != nil {
				<-sem // Release the slot.
				d.logger.Warn("claim task failed", "runtime_id", rid, "error", err)
				continue
			}
			if task != nil {
				d.logger.Info("task received", "task", shortID(task.ID), "issue", task.IssueID)
				wg.Add(1)
				go func(t Task) {
					defer wg.Done()
					defer func() { <-sem }()
					d.handleTask(ctx, t)
				}(*task)
				claimed = true
				pollOffset = (pollOffset + i + 1) % n
				break
			}
			// No task for this runtime, release the slot and try next.
			<-sem
		}

	sleep:
		if !claimed {
			pollCount++
			if pollCount%20 == 1 {
				d.logger.Debug("poll: no tasks", "runtimes", runtimeIDs, "cycle", pollCount)
			}
			pollOffset = (pollOffset + 1) % n
			if err := sleepWithContext(ctx, d.cfg.PollInterval); err != nil {
				wg.Wait()
				return err
			}
		} else {
			pollCount = 0
		}
	}
}

func (d *Daemon) handleTask(ctx context.Context, task Task) {
	defer d.clearTaskBranch(task.ID)

	// Make the task's GitHub token discoverable by the daemon's own git
	// operations (e.g. /repo/checkout) for the duration of this task. The
	// token never leaves the daemon process — agents and the CLI just send
	// task_id, and the daemon looks the token up internally.
	d.rememberTaskToken(task.ID, task.GitHubToken)
	defer d.clearTaskToken(task.ID)

	d.mu.Lock()
	rt := d.runtimeIndex[task.RuntimeID]
	d.mu.Unlock()
	provider := rt.Provider

	// Task-scoped logger with short ID for readable concurrent logs.
	taskLog := d.logger.With("task", shortID(task.ID))
	agentName := "agent"
	if task.Agent != nil {
		agentName = task.Agent.Name
	}
	taskLog.Info("picked task", "issue", task.IssueID, "agent", agentName, "provider", provider)

	if err := d.client.StartTask(ctx, task.ID); err != nil {
		taskLog.Error("start task failed", "error", err)
		if failErr := d.client.FailTask(ctx, task.ID, fmt.Sprintf("start task failed: %s", err.Error())); failErr != nil {
			taskLog.Error("fail task after start error", "error", failErr)
		}
		return
	}

	_ = d.client.ReportProgress(ctx, task.ID, fmt.Sprintf("Launching %s", provider), 1, 2)

	// Create a cancellable context so we can interrupt the running agent
	// when the server-side task status changes to 'cancelled'.
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	// Poll for cancellation every 5 seconds while the task is running.
	cancelledByPoll := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if status, err := d.client.GetTaskStatus(ctx, task.ID); err == nil && status == "cancelled" {
					taskLog.Info("task cancelled by server, interrupting agent")
					runCancel()
					close(cancelledByPoll)
					return
				}
			}
		}
	}()

	result, err := d.runTask(runCtx, task, provider, taskLog)

	// Check if we were cancelled by the polling goroutine.
	select {
	case <-cancelledByPoll:
		taskLog.Info("task cancelled during execution, discarding result")
		return
	default:
	}

	if err != nil {
		taskLog.Error("task failed", "error", err)
		if failErr := d.client.FailTask(ctx, task.ID, err.Error()); failErr != nil {
			taskLog.Error("fail task callback failed", "error", failErr)
		}
		return
	}

	_ = d.client.ReportProgress(ctx, task.ID, "Finishing task", 2, 2)

	// Check if the task was cancelled while it was running (e.g. issue
	// was reassigned). If so, skip reporting results — the server already
	// moved the task to 'cancelled' so complete/fail would fail anyway.
	if status, err := d.client.GetTaskStatus(ctx, task.ID); err == nil && status == "cancelled" {
		taskLog.Info("task cancelled during execution, discarding result")
		return
	}

	if result.PRURL == "" {
		result.PRURL = extractPRURL(result.Comment)
	}
	if result.BranchName == "" {
		result.BranchName = d.consumeTaskBranch(task.ID)
	}

	switch result.Status {
	case "blocked":
		if err := d.client.FailTask(ctx, task.ID, result.Comment); err != nil {
			taskLog.Error("report blocked task failed", "error", err)
		}
	default:
		taskLog.Info("task completed", "status", result.Status)
		if err := d.client.CompleteTask(ctx, task.ID, result.Comment, result.PRURL, result.BranchName, result.SessionID, result.WorkDir); err != nil {
			taskLog.Error("complete task failed, falling back to fail", "error", err)
			if failErr := d.client.FailTask(ctx, task.ID, fmt.Sprintf("complete task failed: %s", err.Error())); failErr != nil {
				taskLog.Error("fail task fallback also failed", "error", failErr)
			}
		}
	}
}

func (d *Daemon) runTask(ctx context.Context, task Task, provider string, taskLog *slog.Logger) (TaskResult, error) {
	entry, ok := d.cfg.Agents[provider]
	if !ok {
		return TaskResult{}, fmt.Errorf("no agent configured for provider %q", provider)
	}

	agentName := "agent"
	var skills []SkillData
	var instructions string
	if task.Agent != nil {
		agentName = task.Agent.Name
		skills = task.Agent.Skills
		instructions = task.Agent.Instructions
	}

	// Prepare isolated execution environment.
	// Repos are passed as metadata only — the agent checks them out on demand
	// via `multica repo checkout <url>`.
	taskCtx := execenv.TaskContextForEnv{
		IssueID:           task.IssueID,
		TriggerCommentID:  task.TriggerCommentID,
		AgentName:         agentName,
		AgentInstructions: instructions,
		AgentSkills:       convertSkillsForEnv(skills),
		Repos:             convertReposForEnv(task.Repos),
		GitHubCodeAccess:  task.GitHubCodeAccess,
	}

	// Try to reuse the workdir from a previous task on the same (agent, issue) pair.
	var env *execenv.Environment
	if task.PriorWorkDir != "" {
		env = execenv.Reuse(task.PriorWorkDir, provider, taskCtx, d.logger)
	}
	if env == nil {
		var err error
		env, err = execenv.Prepare(execenv.PrepareParams{
			WorkspacesRoot: d.cfg.WorkspacesRoot,
			WorkspaceID:    task.WorkspaceID,
			TaskID:         task.ID,
			AgentName:      agentName,
			Provider:       provider,
			Task:           taskCtx,
		}, d.logger)
		if err != nil {
			return TaskResult{}, fmt.Errorf("prepare execution environment: %w", err)
		}
	}

	// Inject runtime-specific config (meta skill) so the agent discovers .agent_context/.
	if err := execenv.InjectRuntimeConfig(env.WorkDir, provider, taskCtx); err != nil {
		d.logger.Warn("execenv: inject runtime config failed (non-fatal)", "error", err)
	}
	// NOTE: No cleanup — workdir is preserved for reuse by future tasks on
	// the same (agent, issue) pair. The work_dir path is stored in DB on
	// task completion and passed back via PriorWorkDir on the next claim.

	prompt := BuildPrompt(task)

	// Pass the daemon's auth credentials and context so the spawned agent CLI
	// can call the Multica API and the local daemon (e.g. `multica repo checkout`).
	agentEnv := map[string]string{
		"MULTICA_TOKEN":        d.client.Token(),
		"MULTICA_SERVER_URL":   d.cfg.ServerBaseURL,
		"MULTICA_DAEMON_PORT":  fmt.Sprintf("%d", d.cfg.HealthPort),
		"MULTICA_WORKSPACE_ID": task.WorkspaceID,
		"MULTICA_AGENT_NAME":   agentName,
		"MULTICA_AGENT_ID":     task.AgentID,
		"MULTICA_TASK_ID":      task.ID,
	}

	// Inject workspace-level provider API key if available.
	// This allows running without per-user CLI auth (e.g. `claude login`).
	if apiKey := d.resolveProviderAPIKey(provider, task.ProviderAPIKey); apiKey != "" {
		d.logger.Info("injecting provider API key", "provider", provider, "key_len", len(apiKey), "from_task", task.ProviderAPIKey != "")
		switch provider {
		case "claude":
			agentEnv["ANTHROPIC_API_KEY"] = apiKey
		case "codex":
			agentEnv["CODEX_API_KEY"] = apiKey
		case "opencode":
			agentEnv["OPENAI_API_KEY"] = apiKey
		case "cursor":
			agentEnv["CURSOR_API_KEY"] = apiKey
		}
	} else {
		d.logger.Warn("no provider API key available", "provider", provider, "task_key_present", task.ProviderAPIKey != "")
	}
	// Point Codex to the per-task CODEX_HOME so it discovers skills natively
	// without polluting the system ~/.codex/skills/.
	if env.CodexHome != "" {
		agentEnv["CODEX_HOME"] = env.CodexHome
		if provider == "codex" || provider == "opencode" {
			key := agentEnv["OPENAI_API_KEY"]
			if provider == "codex" {
				key = agentEnv["CODEX_API_KEY"]
			}
			if key != "" {
				execenv.WriteCodexAuth(env.CodexHome, key, d.logger)
			}
		}
	}
	// Inject GitHub token for authenticated git operations and `gh` CLI.
	if task.GitHubToken != "" {
		agentEnv["GITHUB_TOKEN"] = task.GitHubToken
		agentEnv["GH_TOKEN"] = task.GitHubToken
		if askpassPath, err := execenv.WriteGitAskPass(env.WorkDir, task.GitHubToken); err == nil {
			agentEnv["GIT_ASKPASS"] = askpassPath
			agentEnv["GIT_TERMINAL_PROMPT"] = "0"
		}
	}
	backend, err := agent.New(provider, agent.Config{
		ExecutablePath: entry.Path,
		Env:            agentEnv,
		Logger:         d.logger,
	})
	if err != nil {
		return TaskResult{}, fmt.Errorf("create agent backend: %w", err)
	}

	reused := task.PriorWorkDir != "" && env.WorkDir == task.PriorWorkDir
	taskLog.Info("starting agent",
		"provider", provider,
		"workdir", env.WorkDir,
		"model", entry.Model,
		"reused", reused,
	)
	if task.PriorSessionID != "" {
		taskLog.Info("resuming session", "session_id", task.PriorSessionID)
	}

	taskStart := time.Now()

	session, err := backend.Execute(ctx, prompt, agent.ExecOptions{
		Cwd:             env.WorkDir,
		Model:           entry.Model,
		Timeout:         d.cfg.AgentTimeout,
		ResumeSessionID: task.PriorSessionID,
	})
	if err != nil {
		return TaskResult{}, err
	}

	// Drain message channel — forward to server for live output + log locally.
	var toolCount atomic.Int32
	go func() {
		var seq atomic.Int32
		var mu sync.Mutex
		var pendingText strings.Builder
		var pendingThinking strings.Builder
		var batch []TaskMessageData
		callIDToTool := map[string]string{} // track callID → tool name for tool_result

		flush := func() {
			mu.Lock()
			// Flush any accumulated thinking as a single message.
			if pendingThinking.Len() > 0 {
				s := seq.Add(1)
				batch = append(batch, TaskMessageData{
					Seq:     int(s),
					Type:    "thinking",
					Content: pendingThinking.String(),
				})
				pendingThinking.Reset()
			}
			// Flush any accumulated text as a single message.
			if pendingText.Len() > 0 {
				s := seq.Add(1)
				batch = append(batch, TaskMessageData{
					Seq:     int(s),
					Type:    "text",
					Content: pendingText.String(),
				})
				pendingText.Reset()
			}
			toSend := batch
			batch = nil
			mu.Unlock()

			if len(toSend) > 0 {
				sendCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := d.client.ReportTaskMessages(sendCtx, task.ID, toSend); err != nil {
					taskLog.Debug("failed to report task messages", "error", err)
				}
				cancel()
			}
		}

		// Periodically flush accumulated text/thinking messages.
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		done := make(chan struct{})
		go func() {
			for {
				select {
				case <-ticker.C:
					flush()
				case <-done:
					return
				}
			}
		}()

		for msg := range session.Messages {
			switch msg.Type {
			case agent.MessageToolUse:
				n := toolCount.Add(1)
				taskLog.Info(fmt.Sprintf("tool #%d: %s", n, msg.Tool))
				if msg.CallID != "" {
					mu.Lock()
					callIDToTool[msg.CallID] = msg.Tool
					mu.Unlock()
				}
				s := seq.Add(1)
				mu.Lock()
				batch = append(batch, TaskMessageData{
					Seq:   int(s),
					Type:  "tool_use",
					Tool:  msg.Tool,
					Input: msg.Input,
				})
				mu.Unlock()
			case agent.MessageToolResult:
				s := seq.Add(1)
				output := msg.Output
				if len(output) > 8192 {
					output = output[:8192]
				}
				// Resolve tool name from callID if not set directly.
				toolName := msg.Tool
				if toolName == "" && msg.CallID != "" {
					mu.Lock()
					toolName = callIDToTool[msg.CallID]
					mu.Unlock()
				}
				mu.Lock()
				batch = append(batch, TaskMessageData{
					Seq:    int(s),
					Type:   "tool_result",
					Tool:   toolName,
					Output: output,
				})
				mu.Unlock()
			case agent.MessageThinking:
				if msg.Content != "" {
					mu.Lock()
					pendingThinking.WriteString(msg.Content)
					mu.Unlock()
				}
			case agent.MessageText:
				if msg.Content != "" {
					taskLog.Debug("agent", "text", truncateLog(msg.Content, 200))
					mu.Lock()
					pendingText.WriteString(msg.Content)
					mu.Unlock()
				}
			case agent.MessageError:
				taskLog.Error("agent error", "content", msg.Content)
				s := seq.Add(1)
				mu.Lock()
				batch = append(batch, TaskMessageData{
					Seq:     int(s),
					Type:    "error",
					Content: msg.Content,
				})
				mu.Unlock()
			}
		}

		close(done)
		flush() // Final flush after channel closes.
	}()

	result := <-session.Result
	elapsed := time.Since(taskStart).Round(time.Second)
	taskLog.Info("agent finished",
		"status", result.Status,
		"duration", elapsed.String(),
		"tools", toolCount.Load(),
	)

	switch result.Status {
	case "completed":
		if result.Output == "" {
			return TaskResult{}, fmt.Errorf("%s returned empty output", provider)
		}
		return TaskResult{
			Status:    "completed",
			Comment:   result.Output,
			SessionID: result.SessionID,
			WorkDir:   env.WorkDir,
		}, nil
	case "timeout":
		return TaskResult{}, fmt.Errorf("%s timed out after %s", provider, d.cfg.AgentTimeout)
	default:
		errMsg := result.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("%s execution %s", provider, result.Status)
		}
		return TaskResult{Status: "blocked", Comment: errMsg}, nil
	}
}

func convertReposForEnv(repos []RepoData) []execenv.RepoContextForEnv {
	if len(repos) == 0 {
		return nil
	}
	result := make([]execenv.RepoContextForEnv, len(repos))
	for i, r := range repos {
		result[i] = execenv.RepoContextForEnv{URL: r.URL, Description: r.Description}
	}
	return result
}

// shortID returns the first 8 characters of an ID for readable logs.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// truncateLog truncates a string to maxLen, appending "…" if truncated.
// Also collapses newlines to spaces for single-line log output.
func truncateLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

func convertSkillsForEnv(skills []SkillData) []execenv.SkillContextForEnv {
	if len(skills) == 0 {
		return nil
	}
	result := make([]execenv.SkillContextForEnv, len(skills))
	for i, s := range skills {
		result[i] = execenv.SkillContextForEnv{
			Name:    s.Name,
			Content: s.Content,
		}
		for _, f := range s.Files {
			result[i].Files = append(result[i].Files, execenv.SkillFileContextForEnv{
				Path:    f.Path,
				Content: f.Content,
			})
		}
	}
	return result
}
