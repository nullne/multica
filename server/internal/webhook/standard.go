package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// standardAdapter handles payloads that conform to Multica's own standard schema.
type standardAdapter struct{}

type standardPayload struct {
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Type     string            `json:"type"`
	DedupKey string            `json:"dedup_key"`
	Priority string            `json:"priority"`
	Fields   map[string]string `json:"fields"`
}

func (a *standardAdapter) Parse(payload json.RawMessage, _ http.Header) ([]Event, error) {
	var p standardPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("parse standard payload: %w", err)
	}
	if p.Title == "" {
		return nil, fmt.Errorf("standard payload: title is required")
	}

	eventType := p.Type
	if eventType == "" {
		eventType = "custom"
	}

	data := map[string]string{
		"title": p.Title,
	}
	if p.Body != "" {
		data["body"] = p.Body
	}
	if p.Priority != "" {
		data["priority"] = p.Priority
	}
	for k, v := range p.Fields {
		data["fields."+k] = v
	}
	for _, key := range []string{"source_url", "source_kind", "external_id"} {
		if value := p.Fields[key]; value != "" {
			data[key] = value
		}
	}

	return []Event{{
		Type:       eventType,
		DedupKey:   p.DedupKey,
		Data:       data,
		RawPayload: payload,
	}}, nil
}

func (a *standardAdapter) Keys() []AdapterKey {
	return []AdapterKey{
		{Key: "title", Description: "Event title", Required: true},
		{Key: "body", Description: "Detailed description (markdown)", Required: false},
		{Key: "priority", Description: "Priority hint (urgent, high, medium, low)", Required: false},
		{Key: "fields.*", Description: "Custom key-value pairs from the fields object", Required: false},
	}
}

func (a *standardAdapter) Example() string {
	return `{
  "title": "Deployment failed",
  "body": "Deploy to production failed at step 3",
  "priority": "high",
  "dedup_key": "deploy-prod-v1.2.3",
  "fields": {
    "service": "api-gateway",
    "version": "v1.2.3"
  }
}`
}
