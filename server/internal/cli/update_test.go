package cli

import "testing"

func TestIsReleaseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"0.1.13", true},
		{"v0.1.13", true},
		{" v0.1.13 ", true},
		{"", false},
		{"dev", false},
		{"v0.2.15-235-gdaf0e935", false},
		{"v0.2.15-dirty", false},
		{"0.1", false},
		{"0.1.13.4", false},
		{"a.b.c", false},
	}
	for _, tt := range tests {
		if got := IsReleaseVersion(tt.input); got != tt.want {
			t.Errorf("IsReleaseVersion(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v0.1.14", "v0.1.13", true},
		{"0.1.14", "v0.1.13", true},
		{"v0.2.0", "v0.1.99", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.1.13", "v0.1.13", false},
		{"v0.1.12", "v0.1.13", false},
		// Unparsable sides fail closed — never upgrade off a dev build.
		{"v0.1.14", "v0.1.13-235-gdaf0e935", false},
		{"not-a-version", "v0.1.13", false},
		{"v0.1.14", "", false},
	}
	for _, tt := range tests {
		if got := IsNewerVersion(tt.latest, tt.current); got != tt.want {
			t.Errorf("IsNewerVersion(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}
