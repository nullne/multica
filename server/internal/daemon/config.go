package daemon

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultServerURL             = "ws://localhost:8080/ws"
	DefaultPollInterval          = 3 * time.Second
	DefaultHeartbeatInterval     = 15 * time.Second
	DefaultAgentTimeout          = 2 * time.Hour
	DefaultRuntimeName           = "Local Agent"
	DefaultConfigReloadInterval  = 5 * time.Second
	DefaultWorkspaceSyncInterval = 30 * time.Second
	DefaultHealthPort            = 19514
	DefaultMaxConcurrentTasks    = 20
	// DefaultCodexSemanticInactivityTimeout matches the codex backend's own
	// default; surfaced here so it can be tuned via env without a rebuild.
	DefaultCodexSemanticInactivityTimeout = 10 * time.Minute
	// DefaultAutoUpdateCheckInterval is how often the daemon polls GitHub
	// Releases for a newer CLI version when auto-update is enabled.
	DefaultAutoUpdateCheckInterval = 6 * time.Hour
)

// Config holds all daemon configuration.
type Config struct {
	ServerBaseURL      string
	DaemonID           string
	DeviceName         string
	RuntimeName        string
	CLIVersion         string                // multica CLI version (e.g. "0.1.13")
	Profile            string                // profile name (empty = default)
	Agents             map[string]AgentEntry // "claude" -> entry, "codex" -> entry, "opencode" -> entry
	WorkspacesRoot     string                // base path for execution envs (default: ~/multica_workspaces)
	KeepEnvAfterTask   bool                  // preserve env after task for debugging
	HealthPort         int                   // local HTTP port for health checks (default: 19514)
	MaxConcurrentTasks int                   // max tasks running in parallel (default: 20)
	PollInterval       time.Duration
	HeartbeatInterval  time.Duration
	AgentTimeout       time.Duration
	// CodexSemanticInactivityTimeout bounds how long a codex task may run
	// without semantic progress (tool use, items, status changes) before
	// the daemon gives up on it. Zero falls back to the backend default.
	CodexSemanticInactivityTimeout time.Duration

	// AutoUpdateEnabled controls the periodic self-update loop. Defaults to
	// true for release builds; disable with --no-auto-update or
	// MULTICA_DAEMON_AUTO_UPDATE=false. Dev builds are skipped regardless.
	AutoUpdateEnabled bool
	// AutoUpdateCheckInterval is how often the loop checks GitHub Releases.
	AutoUpdateCheckInterval time.Duration
}

// Overrides allows CLI flags to override environment variables and defaults.
// Zero values are ignored and the env/default value is used instead.
type Overrides struct {
	ServerURL          string
	WorkspacesRoot     string
	PollInterval       time.Duration
	HeartbeatInterval  time.Duration
	AgentTimeout       time.Duration
	MaxConcurrentTasks int
	DaemonID           string
	DeviceName         string
	RuntimeName        string
	Profile            string // profile name (empty = default)
	HealthPort         int    // health check port (0 = use default)
	NoAutoUpdate       bool   // disable the periodic self-update loop
	AutoUpdateInterval time.Duration
}

// LoadConfig builds the daemon configuration from environment variables
// and optional CLI flag overrides.
func LoadConfig(overrides Overrides) (Config, error) {
	// Inherit PATH from the user's login shell so that CLIs installed via
	// bash_profile/zprofile (e.g. multica, ops-cli) are discoverable both
	// during agent probing below and inside spawned agent subprocesses.
	inheritLoginEnv()

	// Server URL: override > env > default
	rawServerURL := envOrDefault("MULTICA_SERVER_URL", DefaultServerURL)
	if overrides.ServerURL != "" {
		rawServerURL = overrides.ServerURL
	}
	serverBaseURL, err := NormalizeServerBaseURL(rawServerURL)
	if err != nil {
		return Config{}, err
	}

	// Probe available agent CLIs
	agents := map[string]AgentEntry{}
	claudePath := envOrDefault("MULTICA_CLAUDE_PATH", "claude")
	if _, err := exec.LookPath(claudePath); err == nil {
		agents["claude"] = AgentEntry{
			Path:  claudePath,
			Model: strings.TrimSpace(os.Getenv("MULTICA_CLAUDE_MODEL")),
		}
	}
	codexPath := envOrDefault("MULTICA_CODEX_PATH", "codex")
	if _, err := exec.LookPath(codexPath); err == nil {
		agents["codex"] = AgentEntry{
			Path:  codexPath,
			Model: strings.TrimSpace(os.Getenv("MULTICA_CODEX_MODEL")),
		}
	}
	opencodePath := envOrDefault("MULTICA_OPENCODE_PATH", "opencode")
	if _, err := exec.LookPath(opencodePath); err == nil {
		agents["opencode"] = AgentEntry{
			Path:  opencodePath,
			Model: strings.TrimSpace(os.Getenv("MULTICA_OPENCODE_MODEL")),
		}
	}
	// Cursor renamed its CLI binary from `agent` to `cursor-agent`; probe the
	// current name first and fall back to the legacy one.
	cursorPath := strings.TrimSpace(os.Getenv("MULTICA_CURSOR_PATH"))
	if cursorPath == "" {
		cursorPath = "cursor-agent"
		if _, err := exec.LookPath(cursorPath); err != nil {
			cursorPath = "agent"
		}
	}
	if _, err := exec.LookPath(cursorPath); err == nil {
		agents["cursor"] = AgentEntry{
			Path:  cursorPath,
			Model: strings.TrimSpace(os.Getenv("MULTICA_CURSOR_MODEL")),
		}
	}
	// Allow starting with zero agents — workspace provider config may trigger
	// auto-install after registration.
	if len(agents) == 0 {
		slog.Warn("no agent CLI found locally; will attempt auto-install from workspace config after registration")
	}

	// Host info
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "local-machine"
	}

	// Durations: override > env > default
	pollInterval, err := durationFromEnv("MULTICA_DAEMON_POLL_INTERVAL", DefaultPollInterval)
	if err != nil {
		return Config{}, err
	}
	if overrides.PollInterval > 0 {
		pollInterval = overrides.PollInterval
	}

	heartbeatInterval, err := durationFromEnv("MULTICA_DAEMON_HEARTBEAT_INTERVAL", DefaultHeartbeatInterval)
	if err != nil {
		return Config{}, err
	}
	if overrides.HeartbeatInterval > 0 {
		heartbeatInterval = overrides.HeartbeatInterval
	}

	agentTimeout, err := durationFromEnv("MULTICA_AGENT_TIMEOUT", DefaultAgentTimeout)
	if err != nil {
		return Config{}, err
	}
	if overrides.AgentTimeout > 0 {
		agentTimeout = overrides.AgentTimeout
	}

	maxConcurrentTasks, err := intFromEnv("MULTICA_DAEMON_MAX_CONCURRENT_TASKS", DefaultMaxConcurrentTasks)
	if err != nil {
		return Config{}, err
	}
	if overrides.MaxConcurrentTasks > 0 {
		maxConcurrentTasks = overrides.MaxConcurrentTasks
	}

	codexSemanticInactivityTimeout, err := durationFromEnv("MULTICA_CODEX_SEMANTIC_INACTIVITY_TIMEOUT", DefaultCodexSemanticInactivityTimeout)
	if err != nil {
		return Config{}, err
	}

	// Auto-update: enabled by default, opt out via flag or env.
	autoUpdateEnabled := true
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MULTICA_DAEMON_AUTO_UPDATE"))) {
	case "false", "0", "no", "off":
		autoUpdateEnabled = false
	}
	if overrides.NoAutoUpdate {
		autoUpdateEnabled = false
	}
	autoUpdateInterval, err := durationFromEnv("MULTICA_DAEMON_AUTO_UPDATE_INTERVAL", DefaultAutoUpdateCheckInterval)
	if err != nil {
		return Config{}, err
	}
	if overrides.AutoUpdateInterval > 0 {
		autoUpdateInterval = overrides.AutoUpdateInterval
	}

	// Profile
	profile := overrides.Profile

	// String overrides
	daemonID := envOrDefault("MULTICA_DAEMON_ID", host)
	if overrides.DaemonID != "" {
		daemonID = overrides.DaemonID
	}
	// Suffix daemon ID with profile name to avoid collisions when multiple
	// daemons register against the same server.
	if profile != "" && !strings.HasSuffix(daemonID, "-"+profile) {
		daemonID = daemonID + "-" + profile
	}

	deviceName := envOrDefault("MULTICA_DAEMON_DEVICE_NAME", host)
	if overrides.DeviceName != "" {
		deviceName = overrides.DeviceName
	}

	runtimeName := envOrDefault("MULTICA_AGENT_RUNTIME_NAME", DefaultRuntimeName)
	if overrides.RuntimeName != "" {
		runtimeName = overrides.RuntimeName
	}

	// Workspaces root: override > env > default (~/multica_workspaces or ~/multica_workspaces_<profile>)
	workspacesRoot := strings.TrimSpace(os.Getenv("MULTICA_WORKSPACES_ROOT"))
	if overrides.WorkspacesRoot != "" {
		workspacesRoot = overrides.WorkspacesRoot
	}
	if workspacesRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("resolve home directory: %w (set MULTICA_WORKSPACES_ROOT to override)", err)
		}
		if profile != "" {
			workspacesRoot = filepath.Join(home, "multica_workspaces_"+profile)
		} else {
			workspacesRoot = filepath.Join(home, "multica_workspaces")
		}
	}
	workspacesRoot, err = filepath.Abs(workspacesRoot)
	if err != nil {
		return Config{}, fmt.Errorf("resolve absolute workspaces root: %w", err)
	}

	// Health port: override > default
	healthPort := DefaultHealthPort
	if overrides.HealthPort > 0 {
		healthPort = overrides.HealthPort
	}

	// Keep env after task: env > default (false)
	keepEnv := os.Getenv("MULTICA_KEEP_ENV_AFTER_TASK") == "true" || os.Getenv("MULTICA_KEEP_ENV_AFTER_TASK") == "1"

	return Config{
		ServerBaseURL:      serverBaseURL,
		DaemonID:           daemonID,
		DeviceName:         deviceName,
		RuntimeName:        runtimeName,
		Profile:            profile,
		Agents:             agents,
		WorkspacesRoot:     workspacesRoot,
		KeepEnvAfterTask:   keepEnv,
		HealthPort:         healthPort,
		MaxConcurrentTasks: maxConcurrentTasks,
		PollInterval:       pollInterval,
		HeartbeatInterval:  heartbeatInterval,
		AgentTimeout:       agentTimeout,

		CodexSemanticInactivityTimeout: codexSemanticInactivityTimeout,

		AutoUpdateEnabled:       autoUpdateEnabled,
		AutoUpdateCheckInterval: autoUpdateInterval,
	}, nil
}

// NormalizeServerBaseURL converts a WebSocket or HTTP URL to a base HTTP URL.
func NormalizeServerBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid MULTICA_SERVER_URL: %w", err)
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
	default:
		return "", fmt.Errorf("MULTICA_SERVER_URL must use ws, wss, http, or https")
	}
	if u.Path == "/ws" {
		u.Path = ""
	}
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}
