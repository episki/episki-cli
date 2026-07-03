package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// CredentialKind tags which credential source the resolver picked.
type CredentialKind int

const (
	CredNone CredentialKind = iota
	CredAPIKeyFlag
	CredAPIKeyEnv
	CredOAuthSession
)

func (k CredentialKind) String() string {
	switch k {
	case CredAPIKeyFlag:
		return "api-key (--api-key)"
	case CredAPIKeyEnv:
		return "api-key (EPISKI_API_KEY)"
	case CredOAuthSession:
		return "oauth-session"
	default:
		return "none"
	}
}

// Credential is the resolved credential plus where it came from. For
// CredOAuthSession the Session field is populated and Token is the
// (possibly-refreshed) Supabase access token.
type Credential struct {
	Kind    CredentialKind
	Token   string
	Session *Session
}

// ErrNoCredentials is returned when nothing in the resolution chain is set.
var ErrNoCredentials = errors.New(
	"no credentials — run `episki auth login` or set EPISKI_API_KEY",
)

// refreshSkew is how close to expiry we'll trigger a refresh. Supabase
// access tokens are 1h by default; refreshing ~60s before expiry leaves
// plenty of margin without thrashing.
const refreshSkew = 60 * time.Second

// Resolve walks the credential chain in this order, refreshing the OAuth
// access token if it's about to expire:
//  1. apiKeyFlag (the value of --api-key, "" if unset)
//  2. EPISKI_API_KEY env var
//  3. Supabase session in the OS keychain (refreshed if expired)
func Resolve(ctx context.Context, apiKeyFlag string) (Credential, error) {
	if apiKeyFlag != "" {
		return Credential{Kind: CredAPIKeyFlag, Token: apiKeyFlag}, nil
	}
	if v := os.Getenv("EPISKI_API_KEY"); v != "" {
		return Credential{Kind: CredAPIKeyEnv, Token: v}, nil
	}
	s, err := LoadSession()
	if err != nil {
		return Credential{}, err
	}
	if s == nil || s.AccessToken == "" {
		return Credential{Kind: CredNone}, ErrNoCredentials
	}

	if s.IsExpired(refreshSkew) {
		refreshed, rerr := RefreshSession(ctx, s)
		if rerr != nil {
			return Credential{}, fmt.Errorf("refresh session: %w", rerr)
		}
		s = refreshed
	}
	return Credential{Kind: CredOAuthSession, Token: s.AccessToken, Session: s}, nil
}
