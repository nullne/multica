package codeagent

// VerifiedProviderVersions are the code-agent CLI versions validated by
// make test-agent-compat. Keep Dockerfile.agent-compat in sync when bumping.
var VerifiedProviderVersions = map[string]string{
	"claude": "2.1.109",
	"codex":  "0.118.0",
	"cursor": "2026.05.16-0338208",
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
