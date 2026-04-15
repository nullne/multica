package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ossAlertAdapter handles Prometheus AlertManager style payloads.
//
// Supports two formats:
//   - Single alert object (original format)
//   - Alertmanager batch format: {"alerts": [...], "status": "firing", ...}
type ossAlertAdapter struct{}

type ossAlertPayload struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     *time.Time        `json:"startsAt"`
	EndsAt       *time.Time        `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
}

type ossAlertBatchPayload struct {
	Alerts []json.RawMessage `json:"alerts"`
}

func (a *ossAlertAdapter) Parse(payload json.RawMessage, _ http.Header) ([]Event, error) {
	var batch ossAlertBatchPayload
	if err := json.Unmarshal(payload, &batch); err == nil && len(batch.Alerts) > 0 {
		var events []Event
		for _, raw := range batch.Alerts {
			ev, err := a.parseSingle(raw)
			if err != nil {
				return nil, err
			}
			events = append(events, ev)
		}
		return events, nil
	}

	ev, err := a.parseSingle(payload)
	if err != nil {
		return nil, err
	}
	return []Event{ev}, nil
}

func (a *ossAlertAdapter) parseSingle(payload json.RawMessage) (Event, error) {
	var p ossAlertPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return Event{}, fmt.Errorf("parse oss-alert payload: %w", err)
	}

	alertName := p.Labels["alertname"]
	if alertName == "" {
		alertName = "Unknown Alert"
	}
	app := p.Labels["app"]

	title := alertName
	if app != "" {
		title = fmt.Sprintf("%s (%s)", alertName, app)
	}

	var body strings.Builder
	if p.StartsAt != nil {
		body.WriteString(fmt.Sprintf("**Started at:** %s\n\n", p.StartsAt.Format(time.RFC3339)))
	}
	if p.EndsAt != nil && !p.EndsAt.IsZero() {
		body.WriteString(fmt.Sprintf("**Ended at:** %s\n\n", p.EndsAt.Format(time.RFC3339)))
	}

	if len(p.Labels) > 0 {
		body.WriteString("**Labels:**\n")
		for k, v := range p.Labels {
			body.WriteString(fmt.Sprintf("- `%s`: %s\n", k, v))
		}
		body.WriteString("\n")
	}

	if len(p.Annotations) > 0 {
		body.WriteString("**Annotations:**\n")
		for k, v := range p.Annotations {
			body.WriteString(fmt.Sprintf("- `%s`: %s\n", k, v))
		}
		body.WriteString("\n")
	}

	if p.GeneratorURL != "" {
		body.WriteString(fmt.Sprintf("[View in monitoring](%s)\n", p.GeneratorURL))
	}

	dedupKey := alertName
	if app != "" {
		dedupKey = alertName + ":" + app
	}

	data := map[string]string{
		"title":     title,
		"body":      body.String(),
		"alertname": alertName,
	}
	if app != "" {
		data["app"] = app
	}
	if p.GeneratorURL != "" {
		data["generator_url"] = p.GeneratorURL
	}

	for k, v := range p.Labels {
		data["labels."+k] = v
	}
	for k, v := range p.Annotations {
		data["annotations."+k] = v
	}

	return Event{
		Type:       "alert.firing",
		DedupKey:   dedupKey,
		Data:       data,
		RawPayload: payload,
	}, nil
}

func (a *ossAlertAdapter) Keys() []AdapterKey {
	return []AdapterKey{
		{Key: "title", Description: "Alert title: alertname (app)", Required: true},
		{Key: "body", Description: "Formatted alert details in markdown", Required: true},
		{Key: "alertname", Description: "Alert name from labels", Required: true},
		{Key: "app", Description: "Application name from labels", Required: false},
		{Key: "generator_url", Description: "Link to the monitoring source", Required: false},
		{Key: "labels.*", Description: "All labels as labels.<key>", Required: false},
		{Key: "annotations.*", Description: "All annotations as annotations.<key>", Required: false},
	}
}

func (a *ossAlertAdapter) Example() string {
	return `{
  "alerts": [
    {
      "labels": {
        "alertname": "ServicePanic",
        "app": "my-service"
      },
      "annotations": {
        "value": "1.2",
        "labels": "map[app:my-service]"
      },
      "startsAt": "2026-04-13T07:39:46.089Z",
      "endsAt": "2026-04-13T07:43:46.089Z",
      "generatorURL": "https://prometheus.example.com/graph?..."
    }
  ],
  "status": "firing"
}`
}
