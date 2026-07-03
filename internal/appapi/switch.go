// Package appapi calls the episki web app's Nitro API. Only one endpoint
// matters to the CLI: switch-workspace, the sole gate that stamps the
// workspace_id claim onto the user's JWT (RLS reads that claim for every
// workspace-scoped row).
package appapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SwitchWorkspace asks the app to stamp workspaceID onto the caller's auth
// user record. The claim only reaches this CLI's JWT on the next token
// refresh, which the caller is responsible for.
//
// The endpoint accepts Authorization: Bearer (added for CLI use); requests
// without an Origin header pass the app's CSRF middleware by design.
func SwitchWorkspace(ctx context.Context, appURL, accessToken, workspaceID string) error {
	body, err := json.Marshal(map[string]string{"workspace_id": workspaceID})
	if err != nil {
		return err
	}
	url := strings.TrimRight(appURL, "/") + "/api/auth/switch-workspace"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("switch-workspace: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}
