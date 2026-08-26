package appapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// This endpoint is the sole gate to a workspace JWT claim, so the request
// shape is a contract with core's server/api/auth/switch-workspace.post.ts:
// POST, a Bearer token (the CLI has no cookie jar), and a {workspace_id}
// body. A drift on any of those reads as "no data" in every later command.
func TestSwitchWorkspaceRequestShape(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotAuth   string
		gotType   string
		gotOrigin string
		gotBody   map[string]string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotType = r.Header.Get("Content-Type")
		gotOrigin = r.Header.Get("Origin")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"workspace_id":"ws-1"}`))
	}))
	defer srv.Close()

	if err := SwitchWorkspace(context.Background(), srv.URL, "token-abc", "ws-1"); err != nil {
		t.Fatalf("SwitchWorkspace: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/auth/switch-workspace" {
		t.Errorf("path = %s, want /api/auth/switch-workspace", gotPath)
	}
	if gotAuth != "Bearer token-abc" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer token-abc")
	}
	if gotType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotType)
	}
	// No Origin header: that is what carries the request past the app's CSRF
	// middleware, and setting one would start failing there.
	if gotOrigin != "" {
		t.Errorf("Origin = %q, want it unset", gotOrigin)
	}
	if gotBody["workspace_id"] != "ws-1" {
		t.Errorf("body = %v, want workspace_id=ws-1", gotBody)
	}
}

// Trailing slashes on a configured app_url must not produce a double slash.
func TestSwitchWorkspaceTrimsAppURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	if err := SwitchWorkspace(context.Background(), srv.URL+"/", "t", "ws-1"); err != nil {
		t.Fatalf("SwitchWorkspace: %v", err)
	}
	if gotPath != "/api/auth/switch-workspace" {
		t.Errorf("path = %q, want no doubled slash", gotPath)
	}
}

// The 403 body ("Not a member of this workspace") is the actionable half of
// the failure, so it has to survive into the returned error.
func TestSwitchWorkspaceSurfacesServerError(t *testing.T) {
	for _, tt := range []struct {
		name string
		code int
		body string
	}{
		{"unauthorized", http.StatusUnauthorized, "Unauthorized"},
		{"not a member", http.StatusForbidden, "Not a member of this workspace"},
		{"server error", http.StatusInternalServerError, "boom"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			err := SwitchWorkspace(context.Background(), srv.URL, "t", "ws-1")
			if err == nil {
				t.Fatalf("SwitchWorkspace on %d = nil error, want an error", tt.code)
			}
			if !strings.Contains(err.Error(), tt.body) {
				t.Errorf("error = %q, want it to carry the server message %q", err, tt.body)
			}
		})
	}
}

func TestSwitchWorkspaceUnreachableApp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	err := SwitchWorkspace(context.Background(), url, "t", "ws-1")
	if err == nil {
		t.Fatal("SwitchWorkspace against a closed server = nil error, want an error")
	}
	// The URL belongs in the message: a wrong EPISKI_APP_URL is the usual cause.
	if !strings.Contains(err.Error(), url) {
		t.Errorf("error = %q, want it to name the url it called", err)
	}
}
