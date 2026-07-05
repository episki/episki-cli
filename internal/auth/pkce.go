package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/episki/episki-cli/internal/config"
	"github.com/pkg/browser"
)

// LoginOptions controls the Supabase Auth PKCE flow.
type LoginOptions struct {
	// Provider overrides config.Supabase.Provider for this login (e.g. "google").
	Provider string
}

// Login runs the Supabase Auth PKCE flow over a loopback redirect, persists
// the resulting tokens to the OS keychain, and returns the new session.
func Login(ctx context.Context, opts LoginOptions) (*Session, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Supabase.URL == "" || cfg.Supabase.AnonKey == "" {
		return nil, errors.New("supabase project is not configured — set [supabase] in ~/.config/episki/config.toml or SUPABASE_URL/SUPABASE_ANON_KEY")
	}
	provider := opts.Provider
	if provider == "" {
		provider = cfg.Supabase.Provider
	}
	if provider == "" {
		return nil, errors.New("no OAuth provider configured — pass --provider or set supabase.provider in your config")
	}

	verifier, challenge, err := pkcePair()
	if err != nil {
		return nil, err
	}
	state, err := randomString(32)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("open loopback listener: %w", err)
	}
	defer listener.Close()
	// No path on the redirect: Supabase matches redirect URLs with glob
	// patterns where `/` is a separator, so the allowlisted
	// `http://127.0.0.1:*` matches host:port but not host:port/callback.
	redirectTo := "http://" + listener.Addr().String()

	authorizeURL := buildSupabaseAuthorizeURL(cfg.Supabase, provider, redirectTo, challenge, state)

	type cbResult struct {
		code string
		err  error
	}
	resultCh := make(chan cbResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errMsg := q.Get("error"); errMsg != "" {
			writeBrowserResponse(w, false, q.Get("error_description"))
			resultCh <- cbResult{err: fmt.Errorf("authorize failed: %s", errMsg)}
			return
		}
		if got := q.Get("state"); got != "" && got != state {
			// Supabase doesn't always echo state back in PKCE; only check
			// when present. The PKCE verifier is the actual binding.
			writeBrowserResponse(w, false, "state mismatch")
			resultCh <- cbResult{err: errors.New("oauth state mismatch")}
			return
		}
		code := q.Get("code")
		if code == "" {
			writeBrowserResponse(w, false, "missing authorization code")
			resultCh <- cbResult{err: errors.New("missing authorization code")}
			return
		}
		writeBrowserResponse(w, true, "")
		resultCh <- cbResult{code: code}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(stderr(), "Opening browser to log in. If it doesn't open, visit:\n  %s\n", authorizeURL)
	if err := browser.OpenURL(authorizeURL); err != nil {
		fmt.Fprintf(stderr(), "(could not open browser automatically: %v)\n", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return nil, res.err
		}
		tok, err := exchangePKCECode(ctx, cfg.Supabase, res.code, verifier)
		if err != nil {
			return nil, err
		}
		session := newSessionFromToken(tok)
		if err := SaveSession(session); err != nil {
			return nil, err
		}
		return session, nil
	}
}

func buildSupabaseAuthorizeURL(s config.Supabase, provider, redirectTo, challenge, state string) string {
	q := url.Values{}
	q.Set("provider", provider)
	q.Set("redirect_to", redirectTo)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	// apikey is required and must be a query param so the browser navigation works.
	q.Set("apikey", s.AnonKey)
	return s.AuthorizeURL() + "?" + q.Encode()
}

// supabaseTokenResp is what /auth/v1/token returns for both grant_type=pkce
// and grant_type=refresh_token.
type supabaseTokenResp struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	TokenType    string           `json:"token_type"`
	ExpiresIn    int              `json:"expires_in"`
	ExpiresAt    int64            `json:"expires_at"`
	User         supabaseAuthUser `json:"user"`
}

type supabaseAuthUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (t supabaseTokenResp) expiresAt() time.Time {
	if t.ExpiresAt > 0 {
		return time.Unix(t.ExpiresAt, 0)
	}
	if t.ExpiresIn > 0 {
		return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	}
	return time.Time{}
}

func newSessionFromToken(t *supabaseTokenResp) *Session {
	s := &Session{
		AccessToken:     t.AccessToken,
		RefreshToken:    t.RefreshToken,
		AccessExpiresAt: t.expiresAt(),
		Email:           t.User.Email,
		UserID:          t.User.ID,
	}
	return s
}

// exchangePKCECode trades the authorization code + verifier for a Supabase
// session.
func exchangePKCECode(ctx context.Context, s config.Supabase, code, verifier string) (*supabaseTokenResp, error) {
	body, _ := json.Marshal(map[string]string{
		"auth_code":     code,
		"code_verifier": verifier,
	})
	return supabaseTokenRequest(ctx, s, "pkce", body)
}

// RefreshSession trades a refresh token for a fresh session and persists it.
// Exposed so the credential resolver can refresh transparently before issuing
// PostgREST calls.
func RefreshSession(ctx context.Context, prev *Session) (*Session, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if prev == nil || prev.RefreshToken == "" {
		return nil, errors.New("no refresh token available; run `episki auth login`")
	}
	body, _ := json.Marshal(map[string]string{
		"refresh_token": prev.RefreshToken,
	})
	tok, err := supabaseTokenRequest(ctx, cfg.Supabase, "refresh_token", body)
	if err != nil {
		return nil, err
	}
	next := newSessionFromToken(tok)
	if next.Email == "" {
		next.Email = prev.Email
	}
	if next.UserID == "" {
		next.UserID = prev.UserID
	}
	if err := SaveSession(next); err != nil {
		return nil, err
	}
	return next, nil
}

func supabaseTokenRequest(ctx context.Context, s config.Supabase, grantType string, body []byte) (*supabaseTokenResp, error) {
	u := s.TokenURL() + "?grant_type=" + grantType
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", s.AnonKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("token request failed: %s", readAuthError(resp))
	}
	var tok supabaseTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, errors.New("token response missing access_token")
	}
	return &tok, nil
}

func pkcePair() (verifier, challenge string, err error) {
	verifier, err = randomString(64)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

// episkiSymbolSVG is the brand mark (rounded hexagon + ring), inlined so the
// loopback pages have no external dependencies.
const episkiSymbolSVG = `<svg viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="episki"><path d="M256 45.39 76.751 150.695v210.61L256 466.61l179.249-105.305v-210.61Z" fill="#1c91fe" stroke="#1c91fe" stroke-width="90.78" stroke-linejoin="round"/><path d="M352.227 256c0-53.145-43.082-96.227-96.227-96.227S159.773 202.855 159.773 256 202.855 352.227 256 352.227 352.227 309.145 352.227 256Z" fill="#043f67" stroke="#fff" stroke-width="36.312" stroke-linejoin="round"/></svg>`

// browserPage is the shell for the loopback success/failure pages. Slots:
// favicon (base64 svg), symbol svg, heading, subtext.
const browserPage = `<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>episki</title>
<link rel="icon" href="data:image/svg+xml;base64,%s">
<style>
  :root { color-scheme: light dark; }
  body {
    margin: 0; min-height: 100vh; display: grid; place-items: center;
    font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
    background: #fbfaf8; color: #1c1c1c;
  }
  main { text-align: center; padding: 2rem 1.5rem; animation: rise .45s ease-out; }
  main svg { width: 64px; height: 64px; margin-bottom: 2.25rem; }
  h1 {
    margin: 0 0 1rem; font-size: 2.75rem; font-weight: 650;
    letter-spacing: -0.03em; line-height: 1.1;
  }
  p { margin: 0 auto; max-width: 26rem; font-size: 1.0625rem; line-height: 1.65; color: #6f6b66; }
  @media (prefers-color-scheme: dark) {
    body { background: #0e1116; color: #f0efed; }
    p { color: #98948e; }
  }
  @keyframes rise { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: none; } }
</style>
<main>
  %s
  <h1>%s</h1>
  <p>%s</p>
</main>
</html>
`

func writeBrowserResponse(w http.ResponseWriter, ok bool, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	favicon := base64.StdEncoding.EncodeToString([]byte(episkiSymbolSVG))
	heading, sub := "You&rsquo;re in.", "The episki CLI is signed in as you.<br>Close this tab and head back to your terminal."
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		heading = "Sign-in failed."
		sub = sanitize(msg) + "<br>Return to your terminal for details."
	}
	_, _ = fmt.Fprintf(w, browserPage, favicon, episkiSymbolSVG, heading, sub)
}

func sanitize(s string) string {
	r := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	return r.Replace(s)
}
