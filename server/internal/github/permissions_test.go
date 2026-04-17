package github

import "testing"

// TestPermissionsForCodeAccess_Matrix asserts the full permission matrix
// for every supported code access level. See PermissionsForCodeAccess for
// the rationale; pull_requests=write is intentionally admin-only.
func TestPermissionsForCodeAccess_Matrix(t *testing.T) {
	cases := []struct {
		level string
		want  map[string]string
	}{
		{
			level: CodeAccessRead,
			want: map[string]string{
				"contents":      PermRead,
				"pull_requests": PermRead,
				"issues":        PermWrite,
				"checks":        PermRead,
				"statuses":      PermRead,
				"metadata":      PermRead,
			},
		},
		{
			level: CodeAccessWrite,
			want: map[string]string{
				"contents":      PermWrite,
				"pull_requests": PermRead,
				"issues":        PermWrite,
				"checks":        PermRead,
				"statuses":      PermRead,
				"metadata":      PermRead,
			},
		},
		{
			level: CodeAccessAdmin,
			want: map[string]string{
				"contents":      PermWrite,
				"pull_requests": PermWrite,
				"issues":        PermWrite,
				"checks":        PermRead,
				"statuses":      PermRead,
				"metadata":      PermRead,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			got := PermissionsForCodeAccess(tc.level)
			for key, want := range tc.want {
				if got[key] != want {
					t.Errorf("level=%s: %s = %q, want %q", tc.level, key, got[key], want)
				}
			}
			if len(got) != len(tc.want) {
				t.Errorf("level=%s: got %d perms, want %d (got=%v)", tc.level, len(got), len(tc.want), got)
			}
		})
	}
}

func TestPermissionsForCodeAccess_Unknown(t *testing.T) {
	perms := PermissionsForCodeAccess("unknown")

	if perms["contents"] != PermRead {
		t.Errorf("unknown level should default to contents=read, got %q", perms["contents"])
	}
	if perms["pull_requests"] != PermRead {
		t.Errorf("unknown level should leave pull_requests=read, got %q", perms["pull_requests"])
	}
}

func TestPermissionsForCodeAccess_BaseAlwaysPresent(t *testing.T) {
	for _, level := range []string{CodeAccessRead, CodeAccessWrite, CodeAccessAdmin} {
		perms := PermissionsForCodeAccess(level)
		for _, key := range []string{"issues", "pull_requests", "checks", "statuses", "metadata"} {
			if _, ok := perms[key]; !ok {
				t.Errorf("level=%s: base permission %q missing", level, key)
			}
		}
	}
}

func TestMergeAllowed(t *testing.T) {
	tests := []struct {
		level    string
		expected bool
	}{
		{CodeAccessRead, false},
		{CodeAccessWrite, false},
		{CodeAccessAdmin, true},
		{"", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		if got := MergeAllowed(tt.level); got != tt.expected {
			t.Errorf("MergeAllowed(%q) = %v, want %v", tt.level, got, tt.expected)
		}
	}
}

func TestValidCodeAccess(t *testing.T) {
	tests := []struct {
		level    string
		expected bool
	}{
		{CodeAccessRead, true},
		{CodeAccessWrite, true},
		{CodeAccessAdmin, true},
		{"", false},
		{"unknown", false},
		{"READ", false},
	}
	for _, tt := range tests {
		if got := ValidCodeAccess(tt.level); got != tt.expected {
			t.Errorf("ValidCodeAccess(%q) = %v, want %v", tt.level, got, tt.expected)
		}
	}
}
