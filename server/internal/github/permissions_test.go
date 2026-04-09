package github

import "testing"

func TestPermissionsForCodeAccess_Read(t *testing.T) {
	perms := PermissionsForCodeAccess(CodeAccessRead)

	if perms["contents"] != PermRead {
		t.Errorf("expected contents=read, got %q", perms["contents"])
	}
	if perms["issues"] != PermWrite {
		t.Errorf("expected issues=write, got %q", perms["issues"])
	}
	if perms["pull_requests"] != PermWrite {
		t.Errorf("expected pull_requests=write, got %q", perms["pull_requests"])
	}
	if perms["checks"] != PermRead {
		t.Errorf("expected checks=read, got %q", perms["checks"])
	}
}

func TestPermissionsForCodeAccess_Write(t *testing.T) {
	perms := PermissionsForCodeAccess(CodeAccessWrite)

	if perms["contents"] != PermWrite {
		t.Errorf("expected contents=write, got %q", perms["contents"])
	}
	if perms["issues"] != PermWrite {
		t.Errorf("base permission issues should be write, got %q", perms["issues"])
	}
}

func TestPermissionsForCodeAccess_Admin(t *testing.T) {
	perms := PermissionsForCodeAccess(CodeAccessAdmin)

	if perms["contents"] != PermWrite {
		t.Errorf("expected contents=write for admin, got %q", perms["contents"])
	}
}

func TestPermissionsForCodeAccess_Unknown(t *testing.T) {
	perms := PermissionsForCodeAccess("unknown")

	if perms["contents"] != PermRead {
		t.Errorf("unknown level should default to contents=read, got %q", perms["contents"])
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
