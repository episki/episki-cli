package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Claims is the subset of the Supabase JWT payload the CLI cares about. The
// workspace claim is stamped by the episki app's switch-workspace endpoint
// and is what `app.current_workspace()` reads for every RLS check — without
// it, all workspace-scoped reads return zero rows.
type Claims struct {
	Email       string `json:"email"`
	Subject     string `json:"sub"`
	AppMetadata struct {
		WorkspaceID string `json:"workspace_id"`
	} `json:"app_metadata"`
}

// DecodeClaims parses a JWT payload without verifying the signature. That is
// fine here: the values are used only for display and client-side routing —
// authorization always happens server-side against the verified token.
func DecodeClaims(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT (expected 3 segments, got %d)", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, fmt.Errorf("parse JWT payload: %w", err)
	}
	return &c, nil
}

// WorkspaceID returns the workspace claim on the given token, or "" if the
// token is unparsable or carries no claim.
func WorkspaceID(token string) string {
	c, err := DecodeClaims(token)
	if err != nil {
		return ""
	}
	return c.AppMetadata.WorkspaceID
}
