package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// Model describes a single LLM model exposed by an agent provider.
// The dropdown groups by Provider when the ID uses the
// `provider/model` form (e.g. "openai/gpt-4o" from opencode).
// Default is a *display* hint: the UI badges the entry the
// runtime advertises as its preferred pick. It has no effect
// at execution time — when agent model config is empty the daemon
// passes "" to the backend so each provider's own CLI resolves its
// own default, which is always closer to what the user's account /
// environment actually supports than a static guess here.
type Model struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider,omitempty"`
	Default  bool   `json:"default,omitempty"`
	// Thinking advertises the runtime's reasoning/effort catalog for this
	// model. nil means the runtime/model has no thinking-level control
	// (or the daemon couldn't discover one); the UI hides its picker. The
	// catalog is per-model because Codex's `codex debug models` is itself
	// per-model and Claude's `--effort` superset has known per-model gaps
	// (`xhigh` is Opus-only, `max` is session-only).
	Thinking *ModelThinking `json:"thinking,omitempty"`
}

// ModelThinking carries the per-model reasoning/effort catalog
// surfaced by an agent runtime. Values are runtime-native — Codex
// emits "none|minimal|low|medium|high|xhigh"; Claude emits
// "low|medium|high|xhigh|max". The frontend renders SupportedLevels
// as-is so what users see matches each CLI's own UI.
type ModelThinking struct {
	SupportedLevels []ThinkingLevel `json:"supported_levels"`
	// DefaultLevel is the value the runtime picks when no override is
	// provided. Empty means "the runtime picks, we don't know" — the
	// UI shows "Default" as a generic option.
	DefaultLevel string `json:"default_level,omitempty"`
}

// ThinkingLevel is one entry in a ModelThinking.SupportedLevels list.
// Value is the literal token passed to the CLI (Claude `--effort <value>`
// or Codex `model_reasoning_effort=<value>`); Label is a display string;
// Description is optional helper copy lifted from the upstream catalog
// when available (Codex's `description` field).
type ThinkingLevel struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// modelCache memoizes dynamic discovery calls so repeated UI loads
// don't re-shell the agent CLI. Entries expire after modelCacheTTL.
type modelCacheEntry struct {
	models    []Model
	expiresAt time.Time
}

var (
	modelCacheMu sync.Mutex
	modelCache   = map[string]modelCacheEntry{}
)

const modelCacheTTL = 60 * time.Second

// ListModels returns the models supported by the given agent provider.
// For providers with a known static catalog it returns the baked-in
// list; for providers with a CLI discovery mechanism (cursor, opencode)
// it shells out with caching and falls back to the static list on failure.
//
// For claude, codex, and opencode, the catalog is augmented with per-model
// thinking-level options discovered from the local CLI. Discovery failures
// silently leave Thinking == nil on each entry, which the UI treats as
// "no picker for this model" rather than blocking model selection.
//
// executablePath lets the caller point at a non-default binary; pass
// "" to use the provider's default name on PATH.
func ListModels(ctx context.Context, providerType, executablePath string) ([]Model, error) {
	switch providerType {
	case "claude":
		models := claudeStaticModels()
		annotateClaudeThinking(ctx, models, executablePath)
		return models, nil
	case "codex":
		models := codexStaticModels()
		annotateCodexThinking(ctx, models, executablePath)
		return models, nil
	case "cursor":
		return cachedDiscovery(discoveryCacheKey(providerType, executablePath), func() ([]Model, error) {
			return discoverCursorModels(ctx, executablePath)
		})
	case "opencode":
		return cachedDiscovery(discoveryCacheKey(providerType, executablePath), func() ([]Model, error) {
			return discoverOpenCodeModels(ctx, executablePath)
		})
	default:
		return nil, fmt.Errorf("unknown agent type: %q", providerType)
	}
}

// ModelSelectionSupported reports whether setting an agent-level model has
// any effect for the given provider. Every built-in provider honours
// `opts.Model` end-to-end.
//
// The hook is retained — rather than inlining `true` at the call sites — so
// a future model-less runtime can opt out in one place, which makes the UI
// render a disabled "Managed by runtime" picker instead of an empty
// dropdown plus a silently-ignored manual-entry field.
func ModelSelectionSupported(providerType string) bool {
	return true
}

// cachedDiscovery invokes fn and caches the result for modelCacheTTL.
func cachedDiscovery(key string, fn func() ([]Model, error)) ([]Model, error) {
	modelCacheMu.Lock()
	if entry, ok := modelCache[key]; ok && time.Now().Before(entry.expiresAt) {
		out := entry.models
		modelCacheMu.Unlock()
		return out, nil
	}
	modelCacheMu.Unlock()

	models, err := fn()
	if err != nil {
		return nil, err
	}

	// Don't cache an empty result. Zero models is almost always a transient
	// failure (discovery CLI timeout, not-logged-in, network blip) rather than
	// a runtime that genuinely has no models; caching it would keep the picker
	// blank for the full TTL even after the cause clears. Skipping the cache
	// lets the next request retry immediately.
	if len(models) == 0 {
		return models, nil
	}

	modelCacheMu.Lock()
	modelCache[key] = modelCacheEntry{models: models, expiresAt: time.Now().Add(modelCacheTTL)}
	modelCacheMu.Unlock()
	return models, nil
}

func discoveryCacheKey(providerType, executablePath string) string {
	if executablePath == "" {
		return providerType
	}
	return providerType + ":" + executablePath
}

// ── Static catalogs ──

// claudeStaticModels reflects the Claude Code CLI's accepted --model
// values. Keep this list short and current; stale entries here
// mislead users more than they help. Default = the latest flagship
// so newly created agents pick it up out of the box.
func claudeStaticModels() []Model {
	return []Model{
		{ID: "claude-fable-5", Label: "Claude Fable 5", Provider: "anthropic", Default: true},
		{ID: "claude-sonnet-4-6", Label: "Claude Sonnet 4.6", Provider: "anthropic"},
		{ID: "claude-opus-4-8", Label: "Claude Opus 4.8", Provider: "anthropic"},
		{ID: "claude-opus-4-7", Label: "Claude Opus 4.7", Provider: "anthropic"},
		{ID: "claude-haiku-4-5-20251001", Label: "Claude Haiku 4.5", Provider: "anthropic"},
		{ID: "claude-opus-4-6", Label: "Claude Opus 4.6", Provider: "anthropic"},
		{ID: "claude-sonnet-4-5", Label: "Claude Sonnet 4.5", Provider: "anthropic"},
	}
}

func codexStaticModels() []Model {
	return []Model{
		{ID: "gpt-5.5", Label: "GPT-5.5", Provider: "openai", Default: true},
		{ID: "gpt-5.5-mini", Label: "GPT-5.5 mini", Provider: "openai"},
		{ID: "gpt-5.4", Label: "GPT-5.4", Provider: "openai"},
		{ID: "gpt-5.4-mini", Label: "GPT-5.4 mini", Provider: "openai"},
		{ID: "gpt-5.3-codex", Label: "GPT-5.3 Codex", Provider: "openai"},
		{ID: "gpt-5", Label: "GPT-5", Provider: "openai"},
		{ID: "o3", Label: "o3", Provider: "openai"},
		{ID: "o3-mini", Label: "o3-mini", Provider: "openai"},
	}
}

// cursorStaticModels is a minimal fallback used when
// `cursor-agent --list-models` isn't available (binary missing,
// offline, etc). The real catalog is fetched dynamically because
// Cursor's model IDs shift often and any static list we ship goes
// stale fast.
func cursorStaticModels() []Model {
	return []Model{
		{ID: "auto", Label: "Auto", Provider: "cursor", Default: true},
	}
}

// ── Dynamic discovery ──

// discoverCursorModels runs `cursor-agent --list-models` and parses
// the `id - Label` rows. Cursor's catalog changes often and ships
// many variants of the same base model (thinking / fast / max
// suffixes) — static baking would be obsolete within weeks. On any
// failure we fall back to the minimal static catalog so the UI
// stays usable when cursor-agent isn't installed on the daemon host.
func discoverCursorModels(ctx context.Context, executablePath string) ([]Model, error) {
	if executablePath == "" {
		executablePath = "cursor-agent"
	}
	if _, err := exec.LookPath(executablePath); err != nil {
		return cursorStaticModels(), nil
	}
	// 15s to match the other network-backed discovery paths; cursor-agent
	// fetches its frequently-changing catalog, so a tight cap can time out
	// and fall back to the minimal static list.
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, executablePath, "--list-models")
	hideAgentWindow(cmd)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return cursorStaticModels(), nil
	}
	models := parseCursorModels(string(out))
	if len(models) == 0 {
		return cursorStaticModels(), nil
	}
	return models, nil
}

// parseCursorModels extracts model IDs from `cursor-agent --list-models`.
// Output format (as of cursor-agent 2026.04):
//
//	Available models
//	<blank>
//	auto - Auto
//	composer-2-fast - Composer 2 Fast (current, default)
//	composer-2 - Composer 2
//	…
//
// The model tagged `(default)` is surfaced as Default=true so the
// UI badge points at cursor's own recommendation rather than a
// hard-coded guess from our catalog.
func parseCursorModels(output string) []Model {
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var models []Model
	seen := map[string]bool{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Row format: "<id> - <label>". Skip the "Available models" header.
		idx := strings.Index(line, " - ")
		if idx <= 0 {
			continue
		}
		id := strings.TrimSpace(line[:idx])
		label := strings.TrimSpace(line[idx+3:])
		if !isModelIdentifier(id) {
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		isDefault := strings.Contains(label, "default")
		// Strip the "(current, default)" suffix from the display
		// label since we surface that through the Default flag.
		if paren := strings.Index(label, "("); paren > 0 {
			label = strings.TrimSpace(label[:paren])
		}
		if label == "" {
			label = id
		}
		models = append(models, Model{
			ID:       id,
			Label:    label,
			Provider: "cursor",
			Default:  isDefault,
		})
	}
	return models
}

// isModelIdentifier reports whether s looks like a model identifier
// (alnum first char, then alnum + `-._/`). Anything that fails it is
// either malformed or a header/decoration line.
func isModelIdentifier(s string) bool {
	if s == "" || strings.HasSuffix(s, ":") {
		return false
	}
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')) {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.' || r == '/':
		default:
			return false
		}
	}
	return true
}

// discoverOpenCodeModels runs `opencode models --verbose` and parses its
// output. The CLI prints `provider/model` rows, followed by JSON metadata
// when verbose mode is enabled; we emit IDs verbatim so what the user sees
// matches what `--model` accepts, and project any model `variants` into the
// thinking-level picker because OpenCode's `run --variant` flag is its
// provider-specific reasoning-effort surface.
// On any failure (CLI missing, parse error, timeout) we fall back to
// an empty list so the creatable UI still works.
func discoverOpenCodeModels(ctx context.Context, executablePath string) ([]Model, error) {
	if executablePath == "" {
		executablePath = "opencode"
	}
	if _, err := exec.LookPath(executablePath); err != nil {
		return []Model{}, nil
	}
	// Newer opencode syncs its hosted free-model catalog over the network on
	// `opencode models`, which can take several seconds; a tight cap times
	// out and returns an empty list while the runtime stays online.
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, executablePath, "models", "--verbose")
	hideAgentWindow(cmd)
	// Parse whatever the verbose command printed, even on a non-zero exit — a
	// stale config entry can make `opencode models` exit non-zero while still
	// listing the resolvable catalog.
	out, _ := cmd.Output()
	models := parseOpenCodeModels(string(out))
	if len(models) == 0 {
		// Verbose yielded nothing usable (unsupported flag, error text, or an
		// empty list). Retry the plain command, which omits the per-model JSON
		// but still prints the IDs.
		cmd = exec.CommandContext(runCtx, executablePath, "models")
		hideAgentWindow(cmd)
		out, _ = cmd.Output()
		models = parseOpenCodeModels(string(out))
	}
	if len(models) == 0 {
		return []Model{}, nil
	}
	return models, nil
}

// parseOpenCodeModels accepts the `opencode models` text output and
// extracts IDs. Non-verbose output is one `provider/model` row per line.
// Verbose output appends a pretty-printed JSON object after each ID; when
// that object contains `variants`, each enabled variant becomes a thinking
// level that the backend later passes through `opencode run --variant`.
func parseOpenCodeModels(output string) []Model {
	lines := strings.Split(output, "\n")
	var models []Model
	indexByID := map[string]int{}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		id := parseOpenCodeModelIDLine(line)
		if id == "" {
			continue
		}
		idx, seen := indexByID[id]
		if !seen {
			provider := ""
			if slash := strings.Index(id, "/"); slash > 0 {
				provider = id[:slash]
			}
			idx = len(models)
			indexByID[id] = idx
			models = append(models, Model{ID: id, Label: id, Provider: provider})
		}

		next := i + 1
		for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
			next++
		}
		if next >= len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[next]), "{") {
			continue
		}
		raw, resumeAt := collectOpenCodeModelJSON(lines, next)
		if json.Valid(raw) {
			annotateOpenCodeModelMetadata(&models[idx], raw)
		}
		i = resumeAt - 1
	}
	return models
}

func parseOpenCodeModelIDLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	id := fields[0]
	if strings.HasPrefix(id, `"`) || strings.HasPrefix(id, "{") || strings.HasPrefix(id, "[") {
		return ""
	}
	if !strings.Contains(id, "/") {
		return ""
	}
	// Skip header rows such as PROVIDER/MODEL.
	if id == strings.ToUpper(id) {
		return ""
	}
	return id
}

func collectOpenCodeModelJSON(lines []string, start int) ([]byte, int) {
	var b strings.Builder
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if i > start && parseOpenCodeModelIDLine(line) != "" {
			return []byte(b.String()), i
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i])
		if json.Valid([]byte(b.String())) {
			return []byte(b.String()), i + 1
		}
	}
	return []byte(b.String()), len(lines)
}

type opencodeModelMetadata struct {
	Reasoning bool                            `json:"reasoning"`
	Variants  map[string]opencodeModelVariant `json:"variants"`
}

type opencodeModelVariant struct {
	Disabled        bool            `json:"disabled"`
	ReasoningEffort string          `json:"reasoningEffort"`
	Thinking        json.RawMessage `json:"thinking"`
}

var opencodeVariantLabel = map[string]string{
	"none":    "None",
	"minimal": "Minimal",
	"low":     "Low",
	"medium":  "Medium",
	"high":    "High",
	"xhigh":   "Extra high",
	"max":     "Max",
}

var opencodeVariantOrder = map[string]int{
	"none":    0,
	"minimal": 1,
	"low":     2,
	"medium":  3,
	"high":    4,
	"xhigh":   5,
	"max":     6,
}

func annotateOpenCodeModelMetadata(model *Model, raw []byte) {
	var meta opencodeModelMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return
	}
	if !meta.Reasoning && !openCodeVariantsLookReasoning(meta.Variants) {
		return
	}
	levels := openCodeThinkingLevelsFromVariants(meta.Variants)
	if len(levels) == 0 {
		return
	}
	model.Thinking = &ModelThinking{SupportedLevels: levels}
}

func openCodeVariantsLookReasoning(variants map[string]opencodeModelVariant) bool {
	for name, variant := range variants {
		if _, known := opencodeVariantOrder[name]; known {
			return true
		}
		if variant.ReasoningEffort != "" || len(variant.Thinking) > 0 {
			return true
		}
	}
	return false
}

func openCodeThinkingLevelsFromVariants(variants map[string]opencodeModelVariant) []ThinkingLevel {
	if len(variants) == 0 {
		return nil
	}
	values := make([]string, 0, len(variants))
	for value, variant := range variants {
		if value == "" || variant.Disabled {
			continue
		}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		left, leftKnown := opencodeVariantOrder[values[i]]
		right, rightKnown := opencodeVariantOrder[values[j]]
		if leftKnown && rightKnown {
			return left < right
		}
		if leftKnown != rightKnown {
			return leftKnown
		}
		return values[i] < values[j]
	})
	levels := make([]ThinkingLevel, 0, len(values))
	for _, value := range values {
		label, ok := opencodeVariantLabel[value]
		if !ok {
			label = strings.Title(strings.ReplaceAll(value, "-", " ")) //nolint:staticcheck
		}
		levels = append(levels, ThinkingLevel{Value: value, Label: label})
	}
	return levels
}
