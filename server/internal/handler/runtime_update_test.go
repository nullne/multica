package handler

import "testing"

func TestParseRuntimeAutoUpdateSettings(t *testing.T) {
	tests := []struct {
		name        string
		raw         []byte
		wantEnabled bool
		wantTarget  string
		wantClients bool
	}{
		{
			name:        "empty settings",
			raw:         nil,
			wantEnabled: false,
			wantTarget:  "latest",
			wantClients: true,
		},
		{
			name:        "boolean enabled",
			raw:         []byte(`{"runtime_auto_update": true}`),
			wantEnabled: true,
			wantTarget:  "latest",
			wantClients: true,
		},
		{
			name:        "object custom values",
			raw:         []byte(`{"runtime_auto_update": {"enabled": false, "target_version": "v1.2.3", "update_clients": false}}`),
			wantEnabled: false,
			wantTarget:  "v1.2.3",
			wantClients: false,
		},
		{
			name:        "object defaults enabled",
			raw:         []byte(`{"runtime_auto_update": {"target_version": "v2.0.0"}}`),
			wantEnabled: true,
			wantTarget:  "v2.0.0",
			wantClients: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRuntimeAutoUpdateSettings(tc.raw)
			if got.Enabled != tc.wantEnabled {
				t.Fatalf("Enabled = %v, want %v", got.Enabled, tc.wantEnabled)
			}
			if got.TargetVersion != tc.wantTarget {
				t.Fatalf("TargetVersion = %q, want %q", got.TargetVersion, tc.wantTarget)
			}
			if got.UpdateClients != tc.wantClients {
				t.Fatalf("UpdateClients = %v, want %v", got.UpdateClients, tc.wantClients)
			}
		})
	}
}

func TestShouldAutoUpdate(t *testing.T) {
	tests := []struct {
		name    string
		current string
		target  string
		want    bool
	}{
		{name: "target newer", current: "0.1.0", target: "0.2.0", want: true},
		{name: "same version", current: "v0.2.0", target: "0.2.0", want: false},
		{name: "target older", current: "0.3.0", target: "0.2.9", want: false},
		{name: "no current version", current: "", target: "1.0.0", want: true},
		{name: "invalid target", current: "1.0.0", target: "latest", want: false},
		{name: "prerelease target", current: "1.2.3", target: "1.2.4-beta.1", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldAutoUpdate(tc.current, tc.target); got != tc.want {
				t.Fatalf("shouldAutoUpdate(%q, %q) = %v, want %v", tc.current, tc.target, got, tc.want)
			}
		})
	}
}

func TestUpdateStoreCreateAutoThrottle(t *testing.T) {
	store := NewUpdateStore()

	first, err := store.CreateAuto("runtime-1", "v1.2.3", true)
	if err != nil {
		t.Fatalf("CreateAuto first call error: %v", err)
	}
	if first == nil {
		t.Fatal("CreateAuto first call returned nil request")
	}

	store.Complete(first.ID, "ok")

	second, err := store.CreateAuto("runtime-1", "v1.2.3", true)
	if err != nil {
		t.Fatalf("CreateAuto second call error: %v", err)
	}
	if second != nil {
		t.Fatal("CreateAuto second call should be throttled and return nil")
	}

	third, err := store.CreateAuto("runtime-1", "v1.2.4", true)
	if err != nil {
		t.Fatalf("CreateAuto third call error: %v", err)
	}
	if third == nil {
		t.Fatal("CreateAuto third call should allow a new target version")
	}
}
