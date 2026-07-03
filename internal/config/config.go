// Package config loads and saves user-level CLI configuration.
//
// Secrets (the user's Supabase access/refresh tokens) live in the OS keychain
// via internal/auth, never in this file's TOML. The Supabase anon key is a
// public value baked into the binary; it identifies the project, not the
// user, and RLS does the actual authorization.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
)

// Config is the on-disk shape at ~/.config/episki/config.toml.
type Config struct {
	Supabase Supabase `toml:"supabase,omitempty"`

	// AppURL is the episki web app origin; `episki workspaces use` calls its
	// /api/auth/switch-workspace endpoint to stamp the workspace JWT claim.
	AppURL string `toml:"app_url,omitempty"`

	// Workspace is display-only bookkeeping written by `episki workspaces
	// use`. The JWT claim is the sole authority for scoping; commands must
	// never use these values as a filter source.
	Workspace Workspace `toml:"workspace,omitempty"`

	LastUpdateCheckUnix int64 `toml:"last_update_check_unix,omitempty"`
}

// Supabase holds the project's public addressing info.
type Supabase struct {
	// URL is the project URL, e.g. "https://abcdef.supabase.co".
	URL string `toml:"url,omitempty"`
	// AnonKey is the project's public API key sent as the `apikey` header on
	// every request — an opaque publishable key (sb_publishable_*). The TOML
	// field keeps the historical name `anon_key`. Safe to ship in the binary;
	// it identifies the project, not the user.
	AnonKey string `toml:"anon_key,omitempty"`
	// Provider is the default OAuth provider used by `episki auth login`,
	// e.g. "google" or "azure". Override per-invocation with `--provider`.
	Provider string `toml:"provider,omitempty"`
}

// Workspace is the last workspace selected via `episki workspaces use`.
type Workspace struct {
	ID   string `toml:"id,omitempty"`
	Name string `toml:"name,omitempty"`
}

// AuthorizeURL returns the Supabase Auth authorize endpoint.
func (s Supabase) AuthorizeURL() string { return s.URL + "/auth/v1/authorize" }

// TokenURL returns the Supabase Auth token endpoint.
func (s Supabase) TokenURL() string { return s.URL + "/auth/v1/token" }

// UserURL returns the Supabase Auth current-user endpoint.
func (s Supabase) UserURL() string { return s.URL + "/auth/v1/user" }

// RestURL returns the PostgREST base URL for the Data API.
func (s Supabase) RestURL() string { return s.URL + "/rest/v1" }

// Defaults returns the built-in defaults used when no config file exists.
// These point at the production episki Supabase project (custom domain
// api.episki.com); the publishable key is browser-safe by design.
func Defaults() Config {
	return Config{
		Supabase: Supabase{
			URL:      "https://api.episki.com",
			AnonKey:  "sb_publishable_aE8XQgfMHNTIlqgo_1ZhCQ_hMj1g0t7",
			Provider: "google",
		},
		AppURL: "https://app.episki.com",
	}
}

var (
	loadOnce sync.Once
	loaded   Config
	loadErr  error
)

// Load reads the config from disk, merging Defaults() with environment
// overrides (SUPABASE_URL, SUPABASE_ANON_KEY). Subsequent calls return the
// cached result.
func Load() (Config, error) {
	loadOnce.Do(func() {
		loaded, loadErr = readFromDisk()
	})
	return loaded, loadErr
}

func readFromDisk() (Config, error) {
	cfg := Defaults()

	path, err := Path()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// First run — fall through with defaults.
	case err != nil:
		return cfg, fmt.Errorf("read config: %w", err)
	default:
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config: %w", err)
		}
	}

	if v := os.Getenv("SUPABASE_URL"); v != "" {
		cfg.Supabase.URL = v
	}
	// SUPABASE_KEY is the base repo's canonical name for the publishable
	// key; SUPABASE_ANON_KEY is kept for back-compat and wins if both are set.
	if v := os.Getenv("SUPABASE_KEY"); v != "" {
		cfg.Supabase.AnonKey = v
	}
	if v := os.Getenv("SUPABASE_ANON_KEY"); v != "" {
		cfg.Supabase.AnonKey = v
	}
	if v := os.Getenv("EPISKI_APP_URL"); v != "" {
		cfg.AppURL = v
	}
	return cfg, nil
}

// Save writes the config back to disk, creating parents as needed.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return err
	}

	loadOnce = sync.Once{}
	return nil
}

// Path returns the absolute path to config.toml, honoring XDG_CONFIG_HOME.
func Path() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "episki", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "episki", "config.toml"), nil
}
