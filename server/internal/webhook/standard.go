package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// standardAdapter handles arbitrary JSON payloads sent to Multica API triggers.
type standardAdapter struct{}

func (a *standardAdapter) Parse(payload json.RawMessage, _ http.Header) ([]Event, error) {
	if !json.Valid(payload) {
		return nil, fmt.Errorf("parse standard payload: invalid JSON")
	}
	data := map[string]string{
		"raw_payload": string(payload),
	}

	eventType := "custom"
	dedupKey := ""
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err == nil && object != nil {
		if value := rawJSONString(object["title"]); value != "" {
			data["title"] = value
		}
		if value := rawJSONString(object["body"]); value != "" {
			data["body"] = value
		}
		if value := rawJSONString(object["priority"]); value != "" {
			data["priority"] = value
		}
		if value := rawJSONString(object["type"]); value != "" {
			eventType = value
		}
		dedupKey = rawJSONString(object["dedup_key"])

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(object["fields"], &fields); err == nil {
			for k, raw := range fields {
				value := rawJSONString(raw)
				if value == "" {
					continue
				}
				data["fields."+k] = value
				switch k {
				case "source_url", "source_kind", "external_id":
					data[k] = value
				}
			}
		}
	}

	return []Event{{
		Type:       eventType,
		DedupKey:   dedupKey,
		Data:       data,
		RawPayload: payload,
	}}, nil
}

func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func (a *standardAdapter) Keys() []AdapterKey {
	return []AdapterKey{
		{Key: "title", Description: "Event title", Required: false},
		{Key: "body", Description: "Detailed description (markdown)", Required: false},
		{Key: "priority", Description: "Priority hint (urgent, high, medium, low)", Required: false},
		{Key: "fields.*", Description: "Custom key-value pairs from the fields object", Required: false},
		{Key: "raw_payload", Description: "Original JSON request body", Required: false},
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
