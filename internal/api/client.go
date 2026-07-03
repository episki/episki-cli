// Package api wires postgrest-go to the resolved credential, so resource
// subcommands don't have to think about headers, the apikey, or token
// refresh.
//
// The user's JWT goes in `Authorization: Bearer ...` and the project's anon
// key goes in `apikey: ...`. PostgREST uses the JWT to populate
// `request.jwt.claims` and `auth.uid()`, which is what RLS policies read.
// No code in this CLI grants permissions — RLS does, on every row.
package api

import (
	"context"
	"errors"

	"github.com/episki/episki-cli/internal/auth"
	"github.com/episki/episki-cli/internal/config"
	"github.com/supabase-community/postgrest-go"
)

// New returns a postgrest-go client wired with the user's credential and the
// project's anon key. It refreshes the OAuth access token if necessary
// before constructing the client.
func New(ctx context.Context, rf *auth.RootFlags) (*postgrest.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Supabase.URL == "" || cfg.Supabase.AnonKey == "" {
		return nil, errors.New("supabase project is not configured — set [supabase] in ~/.config/episki/config.toml")
	}
	apiKey := ""
	if rf != nil {
		apiKey = rf.APIKey
	}
	cred, err := auth.Resolve(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"apikey":        cfg.Supabase.AnonKey,
		"Authorization": "Bearer " + cred.Token,
	}
	// The empty schema argument means "public", which is what we want for
	// the Data API. Per-call schema overrides remain available on the
	// returned client.
	return postgrest.NewClient(cfg.Supabase.RestURL(), "", headers), nil
}
