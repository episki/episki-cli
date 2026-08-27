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

func TestPrepareEvidenceUploadRequestShape(t *testing.T) {
	var (
		gotMethod, gotPath, gotAuth string
		gotBody                     map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"duplicate":false,"artifact_id":"a-1","storage_path":"ws/a-1-f.pdf","upload_token":"tok","content_type":"application/pdf"}`))
	}))
	defer srv.Close()

	got, err := PrepareEvidenceUpload(context.Background(), srv.URL, "jwt-1",
		"f.pdf", "application/pdf", 1234, strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("PrepareEvidenceUpload: %v", err)
	}

	if gotMethod != http.MethodPost || gotPath != "/api/evidence/upload/prepare" {
		t.Errorf("got %s %s, want POST /api/evidence/upload/prepare", gotMethod, gotPath)
	}
	if gotAuth != "Bearer jwt-1" {
		t.Errorf("Authorization = %q, want the user's token", gotAuth)
	}
	// size_bytes must be a number, not a string: core validates it with
	// z.number().int(), so a stringified size is a 400.
	if _, ok := gotBody["size_bytes"].(float64); !ok {
		t.Errorf("size_bytes = %T, want a JSON number", gotBody["size_bytes"])
	}
	if gotBody["checksum"] != strings.Repeat("a", 64) {
		t.Errorf("checksum = %v, want the 64-char digest", gotBody["checksum"])
	}
	if got.ArtifactID != "a-1" || got.UploadToken != "tok" || got.ContentType != "application/pdf" {
		t.Errorf("decoded %+v, want the upload fields populated", got)
	}
}

// An unknown extension leaves mime.TypeByExtension empty. Sending
// "mime_type": "" would fail core's own resolution, so the key is omitted and
// core derives the type from the name instead.
func TestPrepareEvidenceUploadOmitsEmptyMime(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"duplicate":false}`))
	}))
	defer srv.Close()

	if _, err := PrepareEvidenceUpload(context.Background(), srv.URL, "t", "notes", "", 10, "abc"); err != nil {
		t.Fatalf("PrepareEvidenceUpload: %v", err)
	}
	if _, present := gotBody["mime_type"]; present {
		t.Errorf("body carried mime_type=%v, want the key omitted entirely", gotBody["mime_type"])
	}
}

func TestPrepareEvidenceUploadDuplicate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"duplicate":true,"evidence_id":"e-1","evidence_name":"Q3 pen test"}`))
	}))
	defer srv.Close()

	got, err := PrepareEvidenceUpload(context.Background(), srv.URL, "t", "f.pdf", "application/pdf", 1, "abc")
	if err != nil {
		t.Fatalf("PrepareEvidenceUpload: %v", err)
	}
	if !got.Duplicate || got.EvidenceID != "e-1" || got.EvidenceName != "Q3 pen test" {
		t.Errorf("decoded %+v, want the existing record identified", got)
	}
}

// Nitro puts the useful sentence in statusMessage; surfacing the raw body
// instead buries it in a stack blob.
func TestAppErrorPrefersStatusMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"url":"/api/x","statusCode":400,"statusMessage":"Unsupported file type: application/x-tar","message":"..."}`))
	}))
	defer srv.Close()

	_, err := PrepareEvidenceUpload(context.Background(), srv.URL, "t", "f.tar", "application/x-tar", 1, "abc")
	if err == nil {
		t.Fatal("PrepareEvidenceUpload on 400 = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "Unsupported file type: application/x-tar") {
		t.Errorf("error = %q, want it to carry core's statusMessage", err)
	}
}

func TestCompleteEvidenceUploadRequestShape(t *testing.T) {
	var (
		gotPath string
		gotBody map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"ok":true,"artifact_id":"a-1","name":"f.pdf","size_bytes":2048,"mime_type":"application/pdf"}`))
	}))
	defer srv.Close()

	got, err := CompleteEvidenceUpload(context.Background(), srv.URL, "t", "e-1", "a-1", "f.pdf", "application/pdf", "abc")
	if err != nil {
		t.Fatalf("CompleteEvidenceUpload: %v", err)
	}
	if gotPath != "/api/evidence/upload/complete" {
		t.Errorf("path = %s, want /api/evidence/upload/complete", gotPath)
	}
	for _, k := range []string{"evidence_id", "artifact_id", "name", "checksum"} {
		if gotBody[k] == nil || gotBody[k] == "" {
			t.Errorf("body missing %s: %v", k, gotBody)
		}
	}
	// Size comes back from Storage, never from us — the client does not send one.
	if _, present := gotBody["size_bytes"]; present {
		t.Errorf("body carried size_bytes, want Storage's number to be authoritative")
	}
	if got.SizeBytes != 2048 {
		t.Errorf("SizeBytes = %d, want Storage's 2048", got.SizeBytes)
	}
}

func TestUploadSignedBytes(t *testing.T) {
	var (
		gotMethod, gotPath, gotToken, gotType, gotUpsert string
		gotBody                                          []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotToken = r.URL.Query().Get("token")
		gotType = r.Header.Get("Content-Type")
		gotUpsert = r.Header.Get("x-upsert")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"Key":"evidence/ws/a-1-f.pdf"}`))
	}))
	defer srv.Close()

	prep := &PrepareResult{
		ArtifactID:  "a-1",
		StoragePath: "ws-1/a-1-f.pdf",
		UploadToken: "tok en/+",
		ContentType: "application/pdf",
	}
	payload := "%PDF-1.7 fake"
	if err := UploadSignedBytes(context.Background(), srv.URL, "sb_publishable_x",
		prep, strings.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatalf("UploadSignedBytes: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	// The bucket goes in the path, and the path from prepare follows it.
	if gotPath != "/storage/v1/object/upload/sign/evidence/ws-1/a-1-f.pdf" {
		t.Errorf("path = %q, want the bucket-prefixed signed upload path", gotPath)
	}
	// A token with URL-significant characters must survive escaping intact.
	if gotToken != "tok en/+" {
		t.Errorf("token = %q, want it escaped on the wire and decoded whole", gotToken)
	}
	// Core's RESOLVED type, not our guess: Storage enforces the bucket's
	// allowed_mime_types, so uploading anything else is rejected there.
	if gotType != "application/pdf" {
		t.Errorf("Content-Type = %q, want the resolved type from prepare", gotType)
	}
	if gotUpsert != "false" {
		t.Errorf("x-upsert = %q, want false — a signed path is single-use", gotUpsert)
	}
	if string(gotBody) != payload {
		t.Errorf("body = %q, want the file's bytes verbatim", gotBody)
	}
}

func TestUploadSignedBytesSurfacesStorageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"statusCode":"400","error":"InvalidMimeType","message":"mime type text/html is not supported"}`))
	}))
	defer srv.Close()

	prep := &PrepareResult{StoragePath: "ws/a.html", UploadToken: "t", ContentType: "text/html"}
	err := UploadSignedBytes(context.Background(), srv.URL, "k", prep, strings.NewReader("x"), 1)
	if err == nil {
		t.Fatal("UploadSignedBytes on 400 = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %q, want Storage's message", err)
	}
}
