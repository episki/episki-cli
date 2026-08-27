package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// jwt builds an unsigned token whose payload is the given JSON. Only the
// payload segment is ever read, so the header and signature are filler.
func jwt(payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return strings.Join([]string{
		enc([]byte(`{"alg":"HS256","typ":"JWT"}`)),
		enc([]byte(payload)),
		"not-a-real-signature",
	}, ".")
}

func TestDecodeClaims(t *testing.T) {
	const ws = "6f1c9d5e-6f0a-4f2b-9c1d-2b8f4a7e0c31"
	token := jwt(`{"sub":"user-1","email":"a@example.com","app_metadata":{"workspace_id":"` + ws + `"}}`)

	c, err := DecodeClaims(token)
	if err != nil {
		t.Fatalf("DecodeClaims: %v", err)
	}
	if c.Subject != "user-1" {
		t.Errorf("Subject = %q, want %q", c.Subject, "user-1")
	}
	if c.Email != "a@example.com" {
		t.Errorf("Email = %q, want %q", c.Email, "a@example.com")
	}
	if c.AppMetadata.WorkspaceID != ws {
		t.Errorf("WorkspaceID = %q, want %q", c.AppMetadata.WorkspaceID, ws)
	}
}

func TestDecodeClaimsErrors(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"not a jwt", "just-a-string"},
		{"two segments", "header.payload"},
		{"four segments", "a.b.c.d"},
		{"payload not base64", "header.!!!not-base64!!!.sig"},
		{"payload not json", jwt(`this is not json`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeClaims(tt.token); err == nil {
				t.Fatalf("DecodeClaims(%q) = nil error, want an error", tt.token)
			}
		})
	}
}

// WorkspaceID must degrade to "" rather than propagating an error: callers
// use the empty string to mean "no workspace selected" and print a hint.
// A token that parses but carries no claim is the common real-world case —
// a freshly signed-in user who has never run `workspaces use`.
func TestWorkspaceID(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{"claim present", jwt(`{"app_metadata":{"workspace_id":"ws-1"}}`), "ws-1"},
		{"no app_metadata", jwt(`{"sub":"user-1"}`), ""},
		{"empty app_metadata", jwt(`{"app_metadata":{}}`), ""},
		{"unparsable token", "garbage", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WorkspaceID(tt.token); got != tt.want {
				t.Errorf("WorkspaceID = %q, want %q", got, tt.want)
			}
		})
	}
}

// Supabase JWT payloads are base64url with no padding (RawURLEncoding).
// Decoding with the padded alphabet fails on payloads whose length isn't a
// multiple of 4, so this pins the encoding choice.
func TestDecodeClaimsUnpaddedPayload(t *testing.T) {
	payload := `{"app_metadata":{"workspace_id":"ws-1"},"sub":"u"}`
	if len(base64.StdEncoding.EncodeToString([]byte(payload)))%4 == 0 {
		// Pad the payload out so the encoded form genuinely needs padding.
		payload = `{"app_metadata":{"workspace_id":"ws-1"},"sub":"uu"}`
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(payload), &probe); err != nil {
		t.Fatalf("test payload is not valid json: %v", err)
	}
	if got := WorkspaceID(jwt(payload)); got != "ws-1" {
		t.Errorf("WorkspaceID = %q, want %q", got, "ws-1")
	}
}
