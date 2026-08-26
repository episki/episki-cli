package resources

import "testing"

func TestIsUUID(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"6f1c9d5e-6f0a-4f2b-9c1d-2b8f4a7e0c31", true},
		{"6F1C9D5E-6F0A-4F2B-9C1D-2B8F4A7E0C31", true},
		// Refs and slugs are the other half of `get <id|ref>`; every one of
		// these must route to the ref column instead.
		{"AC-2", false},
		{"PCI 8.3.1", false},
		{"acme-corp", false},
		{"", false},
		{"6f1c9d5e6f0a4f2b9c1d2b8f4a7e0c31", false},
		{"6f1c9d5e-6f0a-4f2b-9c1d-2b8f4a7e0c3", false},
		{"6f1c9d5e-6f0a-4f2b-9c1d-2b8f4a7e0c311", false},
		{"zzzzzzzz-6f0a-4f2b-9c1d-2b8f4a7e0c31", false},
		{" 6f1c9d5e-6f0a-4f2b-9c1d-2b8f4a7e0c31 ", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isUUID(tt.in); got != tt.want {
				t.Errorf("isUUID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseDue(t *testing.T) {
	t.Run("bare date", func(t *testing.T) {
		got, err := parseDue("2026-09-01")
		if err != nil {
			t.Fatalf("parseDue: %v", err)
		}
		if got.Format("2006-01-02") != "2026-09-01" {
			t.Errorf("parsed %s, want 2026-09-01", got)
		}
	})

	t.Run("rfc3339 keeps the time", func(t *testing.T) {
		got, err := parseDue("2026-09-01T17:30:00Z")
		if err != nil {
			t.Fatalf("parseDue: %v", err)
		}
		if got.Hour() != 17 || got.Minute() != 30 {
			t.Errorf("parsed %s, want 17:30 preserved", got)
		}
	})

	for _, in := range []string{"", "tomorrow", "09/01/2026", "2026-13-01", "2026-9-1"} {
		t.Run("rejects "+in, func(t *testing.T) {
			if _, err := parseDue(in); err == nil {
				t.Fatalf("parseDue(%q) = nil error, want an error", in)
			}
		})
	}
}
