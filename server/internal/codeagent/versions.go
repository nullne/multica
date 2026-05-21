package codeagent

// ProviderCatalogEntry describes the repository-owned code-agent CLI and model
// choices validated by make test-agent-compat.
type ProviderCatalogEntry struct {
	Version         string
	DefaultModel    string
	SupportedModels []string
}

// ProviderCatalog is the repository-owned catalog of code-agent CLI versions
// and model choices. Keep Dockerfile.agent-compat in sync when bumping versions.
var ProviderCatalog = map[string]ProviderCatalogEntry{
	"claude": {
		Version:         "2.1.109",
		DefaultModel:    "sonnet",
		SupportedModels: []string{"sonnet", "opus", "haiku"},
	},
	"codex": {
		Version:         "0.118.0",
		DefaultModel:    "gpt-5.2-codex",
		SupportedModels: []string{"gpt-5.2-codex", "gpt-5.1-codex", "gpt-5"},
	},
	"cursor": {
		Version:         "2026.05.16-0338208",
		DefaultModel:    "gpt-5.5",
		SupportedModels: []string{"gpt-5.5", "gpt-5.3-codex", "claude-4.6-sonnet"},
	},
	"opencode": {
		DefaultModel:    "gpt-5.2-codex",
		SupportedModels: []string{"gpt-5.2-codex", "gpt-5"},
	},
}

// VerifiedProviderVersions are the code-agent CLI versions validated by
// make test-agent-compat. Keep Dockerfile.agent-compat in sync when bumping.
var VerifiedProviderVersions = map[string]string{
	"claude": ProviderCatalog["claude"].Version,
	"codex":  ProviderCatalog["codex"].Version,
	"cursor": ProviderCatalog["cursor"].Version,
}

// Version returns the repository-owned verified version for a provider.
func Version(provider string) (string, bool) {
	version, ok := VerifiedProviderVersions[provider]
	return version, ok
}

// MustVersion returns the verified version or an empty string for display paths.
func MustVersion(provider string) string {
	version, _ := Version(provider)
	return version
}

// Catalog returns the repository-owned catalog entry for a provider.
func Catalog(provider string) (ProviderCatalogEntry, bool) {
	entry, ok := ProviderCatalog[provider]
	return entry, ok
}

// MustCatalog returns the catalog entry or a zero-value entry for display paths.
func MustCatalog(provider string) ProviderCatalogEntry {
	entry, _ := Catalog(provider)
	return entry
}

// SupportsModel reports whether model is in the provider's tested model list.
func SupportsModel(provider, model string) bool {
	entry, ok := Catalog(provider)
	if !ok {
		return false
	}
	for _, supported := range entry.SupportedModels {
		if supported == model {
			return true
		}
	}
	return false
}
