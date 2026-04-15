//go:build agentcompat

package agent

import "testing"

func TestOpencode_Compat(t *testing.T) {
	runCompatTest(t, "opencode")
}
