package resources

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestField(t *testing.T) {
	row := map[string]any{
		"id":       "abc",
		"count":    float64(42),
		"ratio":    1.5,
		"is_group": true,
		"empty":    nil,
		"updated":  "2026-07-05T18:02:58Z",
		"plain":    "not a timestamp",
		"statuses": map[string]any{"name": "In progress", "category": "started"},
		"subsets":  []any{"a", "b"},
	}
	tests := []struct {
		name string
		path string
		want string
	}{
		{"string", "id", "abc"},
		{"whole number stays whole", "count", "42"},
		{"fractional number", "ratio", "1.5"},
		{"bool", "is_group", "true"},
		{"null becomes empty", "empty", ""},
		{"timestamp collapses to date", "updated", "2026-07-05"},
		{"non-timestamp string is verbatim", "plain", "not a timestamp"},
		{"nested path", "statuses.name", "In progress"},
		{"missing key", "nope", ""},
		{"missing nested key", "statuses.nope", ""},
		{"descend into non-object", "id.name", ""},
		{"array falls back to json", "subsets", `["a","b"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := field(row, tt.path); got != tt.want {
				t.Errorf("field(row, %q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestPrintListTable(t *testing.T) {
	raw := []byte(`[
		{"id":"1","name":"Access control","statuses":{"name":"Open"}},
		{"id":"2","name":"Encryption","statuses":null}
	]`)
	cols := []column{{"id", "id"}, {"name", "name"}, {"status", "statuses.name"}}

	out := captureStdout(t, func() {
		if err := printList(raw, cols, false); err != nil {
			t.Fatalf("printList: %v", err)
		}
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2 rows):\n%s", len(lines), out)
	}
	for _, want := range []string{"ID", "NAME", "STATUS"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("header %q missing %q", lines[0], want)
		}
	}
	if !strings.Contains(lines[1], "Access control") || !strings.Contains(lines[1], "Open") {
		t.Errorf("row 1 = %q, want the name and nested status", lines[1])
	}
	// A null nested object must render as an empty cell, not "<nil>".
	if strings.Contains(lines[2], "nil") {
		t.Errorf("row 2 = %q, want an empty cell for the null status", lines[2])
	}
}

// An empty result set is a normal outcome, not an error, and the "No results."
// line goes to stderr so `--json`-free piping stays clean.
func TestPrintListEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		if err := printList([]byte(`[]`), []column{{"id", "id"}}, false); err != nil {
			t.Fatalf("printList: %v", err)
		}
	})
	if out != "" {
		t.Errorf("stdout = %q, want nothing on stdout for an empty list", out)
	}
}

func TestPrintListJSONPassthrough(t *testing.T) {
	raw := []byte(`[{"id":"1","name":"Access control"}]`)
	out := captureStdout(t, func() {
		if err := printList(raw, []column{{"id", "id"}}, true); err != nil {
			t.Fatalf("printList: %v", err)
		}
	})
	// --json must round-trip the server's payload, not the table projection.
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("--json output is not valid json: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0]["name"] != "Access control" {
		t.Errorf("--json dropped fields: %v", rows)
	}
}

func TestPrintListBadJSON(t *testing.T) {
	if err := printList([]byte(`{"code":"42501"}`), []column{{"id", "id"}}, false); err == nil {
		t.Fatal("printList on a non-array payload = nil error, want an error")
	}
}

// captureStdout swaps os.Stdout for a pipe while fn runs. The print helpers
// write to os.Stdout directly, so this is the seam available without
// reshaping their signatures.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()
	w.Close()
	return <-done
}
