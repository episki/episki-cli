package update

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name            string
		current, latest string
		want            bool
	}{
		{"patch bump", "1.0.0", "1.0.1", true},
		{"minor bump", "1.0.9", "1.1.0", true},
		{"major bump", "1.9.9", "2.0.0", true},
		{"same version", "1.2.3", "1.2.3", false},
		{"older release", "1.2.3", "1.2.2", false},
		{"double-digit segments", "1.9.0", "1.10.0", true},
		{"v prefix on both", "v1.0.0", "v1.0.1", true},
		{"prerelease suffix ignored", "1.0.0", "1.0.1-rc.1", true},
		{"build metadata ignored", "1.0.0", "1.0.0+build.5", false},

		// Never nag from a source build or on unparsable input — a bogus
		// notice is worse than a missed one.
		{"dev build", "dev", "1.0.0", false},
		{"empty current", "", "1.0.0", false},
		{"empty latest", "1.0.0", "", false},
		{"latest not semver", "1.0.0", "banana", false},
		{"too few segments", "1.0.0", "2.0", false},
		{"non-numeric segment", "1.0.0", "1.x.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNewer(tt.current, tt.latest); got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		in   string
		want []int // nil means "should not parse"
	}{
		{"1.2.3", []int{1, 2, 3}},
		{"v1.2.3", []int{1, 2, 3}},
		{"1.2.3-rc.1", []int{1, 2, 3}},
		{"1.2.3+meta", []int{1, 2, 3}},
		{"01.02.03", []int{1, 2, 3}},
		{"1.2", nil},
		{"1.2.3.4", nil},
		{"", nil},
		{"1..3", nil},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := parseSemver(tt.in)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("parseSemver(%q) = %v, want nil", tt.in, got)
				}
				return
			}
			if len(got) != 3 || got[0] != tt.want[0] || got[1] != tt.want[1] || got[2] != tt.want[2] {
				t.Fatalf("parseSemver(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
