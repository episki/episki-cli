// Package update implements the daily "newer version available" notice and
// the `episki upgrade` command.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/episki/episki-cli/internal/config"
)

// CheckOnce prints a one-line stderr notice if a newer release is available,
// at most once per 24 hours. It is safe to call from a goroutine and never
// returns an error to the caller — failures are silent by design.
//
// The notice is suppressed when EPISKI_NO_UPDATE_CHECK=1 or when stderr isn't
// a terminal (i.e., scripted use).
func CheckOnce(ctx context.Context, currentVersion string) {
	if os.Getenv("EPISKI_NO_UPDATE_CHECK") == "1" {
		return
	}
	if !isStderrTerminal() {
		return
	}

	cfg, err := config.Load()
	if err != nil {
		return
	}
	if cfg.LastUpdateCheckUnix > 0 {
		last := time.Unix(cfg.LastUpdateCheckUnix, 0)
		if time.Since(last) < 24*time.Hour {
			return
		}
	}

	latest, err := fetchLatestVersion(ctx)
	if err != nil {
		return
	}

	cfg.LastUpdateCheckUnix = time.Now().Unix()
	_ = config.Save(cfg)

	if isNewer(currentVersion, latest) {
		fmt.Fprintf(os.Stderr,
			"\nA newer episki release is available: %s → %s. Run `episki upgrade` (or set EPISKI_NO_UPDATE_CHECK=1 to silence).\n",
			currentVersion, latest)
	}
}

func fetchLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/episki/episki-cli/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("github releases: %s", resp.Status)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return strings.TrimPrefix(release.TagName, "v"), nil
}

// isNewer compares two semver-ish strings. It tolerates pre-release suffixes
// by ignoring everything after the first "-" and treats unparseable input as
// "not newer" so we never spam users with bogus notices.
func isNewer(current, latest string) bool {
	if current == "" || current == "dev" || latest == "" {
		return false
	}
	c := parseSemver(current)
	l := parseSemver(latest)
	if c == nil || l == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parseSemver(s string) []int {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return nil
	}
	out := make([]int, 3)
	for i, p := range parts {
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				return nil
			}
			n = n*10 + int(r-'0')
		}
		out[i] = n
	}
	return out
}
