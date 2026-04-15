//go:build agentcompat

package agent

import "testing"

func TestCodex_Compat(t *testing.T) {
	runCompatTest(t, "codex")
}
