package config

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate points config at a scratch XDG dir and clears the env overrides so
// each test starts from the built-in defaults.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	for _, k := range []string{"SUPABASE_URL", "SUPABASE_KEY", "SUPABASE_ANON_KEY", "EPISKI_APP_URL"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	return dir
}

func TestPathHonorsXDG(t *testing.T) {
	dir := isolate(t)
	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if want := filepath.Join(dir, "episki", "config.toml"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

// A missing config file is the first-run state, not an error: the baked-in
// production defaults must come back intact.
func TestReadFromDiskMissingFileUsesDefaults(t *testing.T) {
	isolate(t)
	cfg, err := readFromDisk()
	if err != nil {
		t.Fatalf("readFromDisk: %v", err)
	}
	if cfg.Supabase.URL != Defaults().Supabase.URL {
		t.Errorf("URL = %q, want the default %q", cfg.Supabase.URL, Defaults().Supabase.URL)
	}
	if cfg.AppURL != Defaults().AppURL {
		t.Errorf("AppURL = %q, want the default %q", cfg.AppURL, Defaults().AppURL)
	}
}

func TestReadFromDiskMergesFileOverDefaults(t *testing.T) {
	dir := isolate(t)
	writeConfig(t, dir, `
app_url = "http://localhost:3000"

[supabase]
url = "http://127.0.0.1:54321"
`)
	cfg, err := readFromDisk()
	if err != nil {
		t.Fatalf("readFromDisk: %v", err)
	}
	if cfg.Supabase.URL != "http://127.0.0.1:54321" {
		t.Errorf("URL = %q, want the file's value", cfg.Supabase.URL)
	}
	if cfg.AppURL != "http://localhost:3000" {
		t.Errorf("AppURL = %q, want the file's value", cfg.AppURL)
	}
	// Fields the file omits keep the default rather than zeroing out.
	if cfg.Supabase.AnonKey != Defaults().Supabase.AnonKey {
		t.Errorf("AnonKey = %q, want the default to survive a partial file", cfg.Supabase.AnonKey)
	}
}

func TestReadFromDiskEnvOverridesFile(t *testing.T) {
	dir := isolate(t)
	writeConfig(t, dir, "[supabase]\nurl = \"http://from-file\"\n")
	t.Setenv("SUPABASE_URL", "http://from-env")

	cfg, err := readFromDisk()
	if err != nil {
		t.Fatalf("readFromDisk: %v", err)
	}
	if cfg.Supabase.URL != "http://from-env" {
		t.Errorf("URL = %q, want the env override to win", cfg.Supabase.URL)
	}
}

// SUPABASE_KEY is the base repo's canonical name; SUPABASE_ANON_KEY is the
// back-compat alias and is documented as winning when both are set.
func TestReadFromDiskKeyAliasPrecedence(t *testing.T) {
	isolate(t)
	t.Setenv("SUPABASE_KEY", "sb_publishable_canonical")
	t.Setenv("SUPABASE_ANON_KEY", "sb_publishable_legacy")

	cfg, err := readFromDisk()
	if err != nil {
		t.Fatalf("readFromDisk: %v", err)
	}
	if cfg.Supabase.AnonKey != "sb_publishable_legacy" {
		t.Errorf("AnonKey = %q, want SUPABASE_ANON_KEY to win", cfg.Supabase.AnonKey)
	}
}

func TestReadFromDiskBadTOML(t *testing.T) {
	dir := isolate(t)
	writeConfig(t, dir, "this is not = = toml\n")
	if _, err := readFromDisk(); err == nil {
		t.Fatal("readFromDisk on malformed toml = nil error, want an error")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	isolate(t)
	cfg := Defaults()
	cfg.Workspace = Workspace{ID: "ws-1", Name: "Acme"}
	cfg.LastUpdateCheckUnix = 1767225600

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	// The file may hold a workspace name and check timestamps, never secrets:
	// tokens belong in the OS keychain.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %v, want 0600", perm)
	}

	back, err := readFromDisk()
	if err != nil {
		t.Fatalf("readFromDisk: %v", err)
	}
	if back.Workspace.ID != "ws-1" || back.Workspace.Name != "Acme" {
		t.Errorf("workspace round-trip = %+v, want ws-1/Acme", back.Workspace)
	}
	if back.LastUpdateCheckUnix != 1767225600 {
		t.Errorf("LastUpdateCheckUnix = %d, want it preserved", back.LastUpdateCheckUnix)
	}
}

func TestSupabaseURLHelpers(t *testing.T) {
	s := Supabase{URL: "https://api.episki.com"}
	tests := map[string]string{
		s.AuthorizeURL(): "https://api.episki.com/auth/v1/authorize",
		s.TokenURL():     "https://api.episki.com/auth/v1/token",
		s.UserURL():      "https://api.episki.com/auth/v1/user",
		s.RestURL():      "https://api.episki.com/rest/v1",
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "episki"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "episki", "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
