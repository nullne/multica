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

func TestStandardAdapterAcceptsArbitraryJSON(t *testing.T) {
	adapter := &standardAdapter{}
	payload := json.RawMessage(`[
		{"event": "deploy", "status": "failed"},
		{"event": "rollback", "status": "started"}
	]`)
	events, err := adapter.Parse(payload, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	event := events[0]
	if event.Type != "custom" {
		t.Fatalf("event type = %q, want custom", event.Type)
	}
	if string(event.RawPayload) != string(payload) {
		t.Fatalf("raw payload = %s, want %s", event.RawPayload, payload)
	}
	if event.Data["raw_payload"] != string(payload) {
		t.Fatalf("raw_payload data = %q", event.Data["raw_payload"])
	}
}

func TestStandardAdapterRequiresValidJSON(t *testing.T) {
	adapter := &standardAdapter{}
	if _, err := adapter.Parse(json.RawMessage(`{"event":`), nil); err == nil {
		t.Fatal("expected invalid JSON to be rejected")
	}
}
