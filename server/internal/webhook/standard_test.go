package webhook

import (
	"encoding/json"
	"testing"
)

func TestStandardAdapterPromotesSourceFields(t *testing.T) {
	adapter := &standardAdapter{}
	events, err := adapter.Parse(json.RawMessage(`{
		"title": "Deployment failed",
		"dedup_key": "deploy-123",
		"fields": {
			"service": "api",
			"source_url": "https://alerts.example.com/incidents/42",
			"source_kind": "alert",
			"external_id": "incident-42"
		}
	}`), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	event := events[0]
	if event.DedupKey != "deploy-123" {
		t.Fatalf("dedup key = %q", event.DedupKey)
	}
	if event.Data["fields.service"] != "api" {
		t.Fatalf("fields.service = %q", event.Data["fields.service"])
	}
	for key, want := range map[string]string{
		"source_url":  "https://alerts.example.com/incidents/42",
		"source_kind": "alert",
		"external_id": "incident-42",
	} {
		if got := event.Data[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}
