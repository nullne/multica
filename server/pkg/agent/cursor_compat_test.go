//go:build agentcompat

package agent

import "testing"

func TestCursor_Compat(t *testing.T) {
	runCompatTest(t, "cursor")
}
