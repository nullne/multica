package service

import "encoding/json"

// AgentTriggerSnapshot is one entry in an agent's trigger config JSON.
type AgentTriggerSnapshot struct {
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config"`
}

// AgentHasTriggerEnabled checks if a trigger type is enabled in the agent's
// trigger config. Returns true (default-enabled) when the triggers list is
// empty or does not contain the requested type — for backwards compatibility
// with agents created before explicit trigger config was introduced.
func AgentHasTriggerEnabled(raw []byte, triggerType string) bool {
	if len(raw) == 0 {
		return true
	}

	var triggers []AgentTriggerSnapshot
	if err := json.Unmarshal(raw, &triggers); err != nil {
		return false
	}
	if len(triggers) == 0 {
		return true // Empty array = default-enabled (backwards compat)
	}
	for _, trigger := range triggers {
		if trigger.Type == triggerType {
			return trigger.Enabled
		}
	}
	return true // Trigger type not configured = enabled by default
}
