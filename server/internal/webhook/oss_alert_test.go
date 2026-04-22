package webhook

import (
	"encoding/json"
	"testing"
)

func TestOssAlertAdapter_ParseSingle(t *testing.T) {
	adapter := &ossAlertAdapter{}
	payload := json.RawMessage(`{
		"labels": {"alertname": "HighMemory", "app": "api-gateway"},
		"annotations": {"summary": "Memory usage above 90%"},
		"startsAt": "2026-04-15T10:00:00Z",
		"generatorURL": "https://prometheus.example.com/graph?g0.expr=mem"
	}`)

	events, err := adapter.Parse(payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev.Type != "alert.firing" {
		t.Errorf("expected type alert.firing, got %s", ev.Type)
	}
	if ev.DedupKey != "HighMemory:api-gateway" {
		t.Errorf("expected dedup key HighMemory:api-gateway, got %s", ev.DedupKey)
	}
	if ev.Data["title"] != "HighMemory (api-gateway)" {
		t.Errorf("expected title 'HighMemory (api-gateway)', got %s", ev.Data["title"])
	}
	if ev.Data["alertname"] != "HighMemory" {
		t.Errorf("expected alertname HighMemory, got %s", ev.Data["alertname"])
	}
	if ev.Data["app"] != "api-gateway" {
		t.Errorf("expected app api-gateway, got %s", ev.Data["app"])
	}
	if ev.Data["generator_url"] != "https://prometheus.example.com/graph?g0.expr=mem" {
		t.Errorf("unexpected generator_url: %s", ev.Data["generator_url"])
	}
	if ev.Data["labels.alertname"] != "HighMemory" {
		t.Errorf("expected labels.alertname HighMemory, got %s", ev.Data["labels.alertname"])
	}
	if ev.Data["annotations.summary"] != "Memory usage above 90%" {
		t.Errorf("expected annotations.summary, got %s", ev.Data["annotations.summary"])
	}
}

func TestOssAlertAdapter_ParseBatch(t *testing.T) {
	adapter := &ossAlertAdapter{}
	payload := json.RawMessage(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {"alertname": "HighMemory", "app": "api-gateway"},
				"annotations": {"summary": "Memory above 90%"},
				"startsAt": "2026-04-15T10:00:00Z"
			},
			{
				"labels": {"alertname": "ServicePanic", "app": "user-service"},
				"annotations": {"value": "3"},
				"startsAt": "2026-04-15T10:01:00Z",
				"generatorURL": "https://prometheus.example.com/graph"
			}
		]
	}`)

	events, err := adapter.Parse(payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Data["alertname"] != "HighMemory" {
		t.Errorf("event 0: expected alertname HighMemory, got %s", events[0].Data["alertname"])
	}
	if events[0].DedupKey != "HighMemory:api-gateway" {
		t.Errorf("event 0: expected dedup key HighMemory:api-gateway, got %s", events[0].DedupKey)
	}

	if events[1].Data["alertname"] != "ServicePanic" {
		t.Errorf("event 1: expected alertname ServicePanic, got %s", events[1].Data["alertname"])
	}
	if events[1].Data["app"] != "user-service" {
		t.Errorf("event 1: expected app user-service, got %s", events[1].Data["app"])
	}
	if events[1].Data["generator_url"] != "https://prometheus.example.com/graph" {
		t.Errorf("event 1: unexpected generator_url: %s", events[1].Data["generator_url"])
	}
}

func TestOssAlertAdapter_ParseBatchEmpty(t *testing.T) {
	adapter := &ossAlertAdapter{}
	payload := json.RawMessage(`{
		"labels": {"alertname": "NoApp"},
		"startsAt": "2026-04-15T10:00:00Z"
	}`)

	events, err := adapter.Parse(payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].DedupKey != "NoApp" {
		t.Errorf("expected dedup key NoApp, got %s", events[0].DedupKey)
	}
}
