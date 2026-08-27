package resources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"

	"github.com/episki/episki-cli/internal/appapi"
	"github.com/episki/episki-cli/internal/auth"
	"github.com/spf13/cobra"
)

// evidenceUploadCmd puts a local file into the workspace's evidence bucket.
//
// The three steps mirror what the web app does, and for the same reason: the
// bytes cannot go through the app's API (a serverless function body caps well
// below the 50 MiB the bucket allows), and they cannot go straight to Storage
// either (the evidence bucket has no `authenticated` policies). So the app
// mints a signed URL, the bytes go to Storage directly, and the app then
// confirms and records what landed.
func evidenceUploadCmd(rf *auth.RootFlags) *cobra.Command {
	var (
		name         string
		evidenceType string
		source       string
		evidenceID   string
		jsonOut      bool
	)
	cmd := &cobra.Command{
		Use:   "upload <file>",
		Short: "Upload a file as evidence in the active workspace",
		Long: "Uploads a file to the workspace's evidence store.\n\n" +
			"By default a new evidence record is created and named after the file;\n" +
			"pass --evidence to attach the file to an existing record instead.\n\n" +
			"Files are de-duplicated by SHA-256 across the workspace: uploading bytes\n" +
			"that are already here reports the existing record and uploads nothing.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := connect(ctx, rf)
			if err != nil {
				return err
			}
			ws, err := c.requireWorkspace()
			if err != nil {
				return err
			}
			if c.cred.Token == "" {
				return fmt.Errorf("evidence upload needs a signed-in session — run `episki auth login`")
			}

			path := args[0]
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return fmt.Errorf("%s is a directory — upload a single file", path)
			}
			if info.Size() == 0 {
				return fmt.Errorf("%s is empty", path)
			}
			if info.Size() > appapi.EvidenceMaxBytes {
				return fmt.Errorf("%s is %s — the evidence bucket's limit is %s",
					path, humanBytes(info.Size()), humanBytes(appapi.EvidenceMaxBytes))
			}

			fileName := name
			if fileName == "" {
				fileName = filepath.Base(path)
			}
			checksum, err := fileChecksum(path)
			if err != nil {
				return err
			}
			mimeType := mime.TypeByExtension(filepath.Ext(path))

			// Step 1 — is this new, and where should it go? Runs before any
			// evidence row is created, which is what lets the duplicate answer
			// be "use this record" rather than "delete the one you just made".
			prep, err := appapi.PrepareEvidenceUpload(ctx, c.cfg.AppURL, c.cred.Token,
				fileName, mimeType, info.Size(), checksum)
			if err != nil {
				return err
			}
			if prep.Duplicate {
				if jsonOut {
					return printJSON(mustJSON(map[string]any{
						"duplicate":     true,
						"evidence_id":   prep.EvidenceID,
						"evidence_name": prep.EvidenceName,
					}))
				}
				fmt.Fprintf(cmd.ErrOrStderr(),
					"Already in this workspace as %q (%s) — nothing uploaded.\n",
					prep.EvidenceName, prep.EvidenceID)
				return nil
			}

			// Step 2 — the bytes, straight to Storage on the signed URL.
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if err := appapi.UploadSignedBytes(ctx, c.cfg.Supabase.URL, c.cfg.Supabase.AnonKey,
				prep, f, info.Size()); err != nil {
				return err
			}

			// The evidence record the artifact hangs off. Created only after
			// the bytes are safely stored, so a failed upload leaves nothing
			// behind. Attaching to an existing record skips this entirely.
			targetID := evidenceID
			created := false
			if targetID == "" {
				targetID, err = createEvidenceRow(c, ws, fileName, evidenceType, source)
				if err != nil {
					return fmt.Errorf("bytes uploaded, but creating the evidence record failed: %w", err)
				}
				created = true
			}

			// Step 3 — confirm and record. Core verifies the object is really
			// there and reads its size from Storage rather than from us.
			done, err := appapi.CompleteEvidenceUpload(ctx, c.cfg.AppURL, c.cred.Token,
				targetID, prep.ArtifactID, fileName, mimeType, checksum)
			if err != nil {
				if created {
					// Say what exists, so the retry is informed: the record is
					// real and empty, and re-running would make a second one.
					return fmt.Errorf("%w\nThe file uploaded and evidence record %s was created, "+
						"but they were not linked — retry with `--evidence %s`", err, targetID, targetID)
				}
				return err
			}

			if jsonOut {
				return printJSON(mustJSON(map[string]any{
					"evidence_id": targetID,
					"artifact_id": done.ArtifactID,
					"name":        done.Name,
					"size_bytes":  done.SizeBytes,
					"mime_type":   done.MimeType,
				}))
			}
			verb := "Attached to"
			if created {
				verb = "Uploaded as"
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "%s evidence %s\n  %s (%s, %s)\n",
				verb, targetID, done.Name, done.MimeType, humanBytes(done.SizeBytes))
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Name for the evidence record (default: the file name)")
	cmd.Flags().StringVar(&evidenceType, "type", "", "Evidence type (e.g. export, attestation, screenshot)")
	cmd.Flags().StringVar(&source, "source", "", "Where the evidence came from (e.g. a CI job or tool name)")
	cmd.Flags().StringVar(&evidenceID, "evidence", "", "Attach to this existing evidence record instead of creating one")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit the result as JSON")
	return cmd
}

// createEvidenceRow inserts the evidence record through PostgREST, so RLS is
// what decides whether the caller may write to this workspace. workspace_id
// defaults to the JWT claim server-side; sending it explicitly keeps the
// client-side filter convention used by every other command here.
func createEvidenceRow(c *conn, ws, name, evidenceType, source string) (string, error) {
	row := map[string]any{"workspace_id": ws, "name": name}
	if evidenceType != "" {
		row["evidence_type"] = evidenceType
	}
	if source != "" {
		row["source"] = source
	}
	raw, _, err := c.client.From("evidence").
		Insert(row, false, "", "representation", "").
		Execute()
	if err != nil {
		return "", err
	}
	var rows []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return "", fmt.Errorf("decode created evidence: %w", err)
	}
	if len(rows) == 0 || rows[0].ID == "" {
		return "", fmt.Errorf("no evidence record created — check your permissions in this workspace")
	}
	return rows[0].ID, nil
}

// fileChecksum returns the lowercase hex SHA-256 of a file, streaming it so a
// 50 MiB upload never has to be held in memory twice.
func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// humanBytes formats a size the way the limit is discussed (MiB), not the way
// disks are sold.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	return b
}
