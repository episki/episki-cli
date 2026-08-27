package appapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// EvidenceBucket is the private bucket every evidence artifact lands in,
// mirroring EVIDENCE_BUCKET in core's shared/utils/evidenceUpload.ts.
const EvidenceBucket = "evidence"

// EvidenceMaxBytes mirrors EVIDENCE_MAX_BYTES — and, more importantly, the
// evidence bucket's own file_size_limit. Checking it client-side turns a
// wasted upload of a too-large file into an instant, specific error.
const EvidenceMaxBytes = 50 * 1024 * 1024

// PrepareResult is the answer to "should these bytes be uploaded at all, and
// where to?". Exactly one of Duplicate or the upload fields is meaningful.
type PrepareResult struct {
	Duplicate    bool   `json:"duplicate"`
	EvidenceID   string `json:"evidence_id"`
	EvidenceName string `json:"evidence_name"`

	ArtifactID  string `json:"artifact_id"`
	StoragePath string `json:"storage_path"`
	UploadToken string `json:"upload_token"`
	// ContentType is the type core RESOLVED, which is the one the bytes must
	// be uploaded with. Storage enforces the bucket's allowed_mime_types, so
	// sending our own guess instead can be accepted here and rejected there.
	ContentType string `json:"content_type"`
}

// CompleteResult is what core recorded: the artifact row and its edge to the
// evidence record.
type CompleteResult struct {
	OK         bool   `json:"ok"`
	ArtifactID string `json:"artifact_id"`
	Name       string `json:"name"`
	SizeBytes  int64  `json:"size_bytes"`
	MimeType   string `json:"mime_type"`
}

// PrepareEvidenceUpload asks the app whether these bytes are new, and for a
// signed URL to put them at if so. The bytes do not pass through it — a
// serverless function body caps well below the 50 MiB the bucket allows.
func PrepareEvidenceUpload(ctx context.Context, appURL, token, name, mimeType string, size int64, checksum string) (*PrepareResult, error) {
	body := map[string]any{
		"name":       name,
		"size_bytes": size,
		"checksum":   checksum,
	}
	if mimeType != "" {
		body["mime_type"] = mimeType
	}
	var out PrepareResult
	if err := appPostJSON(ctx, appURL, "/api/evidence/upload/prepare", token, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CompleteEvidenceUpload confirms the bytes landed and records the artifact
// plus its edge to the evidence record. Core re-derives the storage path from
// the authenticated workspace rather than trusting one sent back, and reads
// the size from Storage rather than from us.
func CompleteEvidenceUpload(ctx context.Context, appURL, token, evidenceID, artifactID, name, mimeType, checksum string) (*CompleteResult, error) {
	body := map[string]any{
		"evidence_id": evidenceID,
		"artifact_id": artifactID,
		"name":        name,
		"checksum":    checksum,
	}
	if mimeType != "" {
		body["mime_type"] = mimeType
	}
	var out CompleteResult
	if err := appPostJSON(ctx, appURL, "/api/evidence/upload/complete", token, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadSignedBytes puts the file at the signed path Storage minted for it.
//
// This is the wire form of storage-js's uploadToSignedUrl: a PUT to
// /object/upload/sign/<bucket>/<path> with the one-shot token on the query
// string. The token is the authorization here — no user JWT is involved —
// which is exactly why the evidence bucket can stay closed to `authenticated`
// while uploads still work.
func UploadSignedBytes(ctx context.Context, supabaseURL, apiKey string, p *PrepareResult, r io.Reader, size int64) error {
	endpoint := strings.TrimRight(supabaseURL, "/") +
		"/storage/v1/object/upload/sign/" + EvidenceBucket + "/" + p.StoragePath +
		"?token=" + url.QueryEscape(p.UploadToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, r)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("apikey", apiKey)
	req.Header.Set("Content-Type", p.ContentType)
	req.Header.Set("Cache-Control", "max-age=3600")
	req.Header.Set("x-upsert", "false")

	// Generous: 50 MiB over a slow uplink is minutes, and the ctx deadline the
	// caller set is the real bound.
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload bytes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("upload bytes: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

// appPostJSON posts a JSON body to an app route with the user's access token
// and decodes the response. Requests carry no Origin header, which is what
// takes them past the app's CSRF middleware.
func appPostJSON(ctx context.Context, appURL, path, token string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(appURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s: %s", strings.TrimPrefix(path, "/api/"), appError(resp))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// appError pulls the human half out of a Nitro error body, which puts the
// useful sentence in statusMessage and a stack-ish blob around it.
func appError(resp *http.Response) string {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var e struct {
		StatusMessage string `json:"statusMessage"`
		Message       string `json:"message"`
	}
	if json.Unmarshal(raw, &e) == nil {
		for _, m := range []string{e.StatusMessage, e.Message} {
			if m != "" {
				return fmt.Sprintf("%s: %s", resp.Status, m)
			}
		}
	}
	return fmt.Sprintf("%s: %s", resp.Status, strings.TrimSpace(string(raw)))
}
