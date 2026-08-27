package resources

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/supabase-community/postgrest-go"
)

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

// filterArchived is the one query fragment in this package that a dependency
// bump has already broken once: postgrest-go v0.0.12 turned Not() into a call
// that rejects itself ("invalid Filter operator: not.is"), which silently cost
// us `work-items list --archived`. These assert the wire format rather than
// the builder call, so the next bump either keeps emitting this query or
// fails here.
func TestFilterArchivedEmitsQuery(t *testing.T) {
	tests := []struct {
		name     string
		archived bool
		want     string // substring the raw query must contain
	}{
		{"active items", false, "archived_at=is.null"},
		{"archived items", true, "or=%28archived_at.not.is.null%29"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			defer srv.Close()

			client := postgrest.NewClient(srv.URL, "", map[string]string{})
			q := client.From("work_items").Select("id", "", false).Eq("workspace_id", "ws-1")
			if _, _, err := filterArchived(q, tt.archived).Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !strings.Contains(gotQuery, tt.want) {
				t.Errorf("query = %q, want it to contain %q", gotQuery, tt.want)
			}
			// The two modes must stay mutually exclusive: an archived listing
			// that also carries archived_at=is.null returns nothing at all.
			if tt.archived && strings.Contains(gotQuery, "archived_at=is.null") {
				t.Errorf("query = %q, want no is.null filter on an archived listing", gotQuery)
			}
		})
	}
}
