package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Event is the normalized output from an adapter. Each adapter converts its
// source-specific payload into one or more of these.
//
// All template variables live in Data as a flat map. Adapters use bare keys
// for deterministic fields they generate (e.g. "title", "body") and prefixed
// keys for dynamic external data (e.g. "labels.xxx", "fields.xxx") to avoid
// namespace collisions.
type Event struct {
	Type       string            // event type: "alert.firing", "alert.resolved", "custom"
	DedupKey   string            // dedup identifier to prevent duplicate processing
	Data       map[string]string // all template variables (flat key-value)
	RawPayload json.RawMessage   // original payload for audit logging
}

// AdapterKey describes a template variable an adapter produces.
type AdapterKey struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// Adapter converts a source-specific payload into normalized webhook events.
type Adapter interface {
	Parse(payload json.RawMessage, headers http.Header) ([]Event, error)
	Keys() []AdapterKey
	Example() string // example JSON payload for documentation
}

// AdapterInfo describes a registered adapter for API/frontend consumption.
type AdapterInfo struct {
	SourceType  string       `json:"source_type"`
	Description string       `json:"description"`
	Keys        []AdapterKey `json:"keys"`
	Example     string       `json:"example"`
}

var adapters = map[string]Adapter{
	"standard":  &standardAdapter{},
	"oss-alert": &ossAlertAdapter{},
	"github":    &githubAdapter{},
}

// GetAdapter returns the adapter for a given source type.
func GetAdapter(sourceType string) (Adapter, error) {
	a, ok := adapters[sourceType]
	if !ok {
		return nil, fmt.Errorf("unsupported webhook source type: %s", sourceType)
	}
	return a, nil
}

// ListAdapters returns info about all registered adapters.
func ListAdapters() []AdapterInfo {
	descriptions := map[string]string{
		"standard":  "Multica standard format. Send events using our schema — no adapter needed.",
		"oss-alert": "Prometheus AlertManager style alerts. Parses labels, annotations, startsAt, endsAt, generatorURL.",
		"github":    "GitHub App webhook events. Receives push, pull_request, issues, and issue_comment events via /api/webhook/github.",
	}
	order := []string{"standard", "oss-alert", "github"}
	result := make([]AdapterInfo, 0, len(order))
	for _, name := range order {
		a := adapters[name]
		result = append(result, AdapterInfo{
			SourceType:  name,
			Description: descriptions[name],
			Keys:        a.Keys(),
			Example:     a.Example(),
		})
	}
	return result
}
