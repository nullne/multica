//go:build agentcompat

package agent

import "testing"

func TestClaude_Compat(t *testing.T) {
	runCompatTest(t, "claude")
}
