package auth

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/episki/episki-cli/internal/config"
)

// LoginWithMagicLink signs in by emailing a magic link whose redirect points
// at a loopback listener, so clicking it hands the PKCE code straight to the
// CLI. This avoids the OAuth-provider redirect path entirely; the emailed
// /auth/v1/verify link honors loopback redirect_to values directly.
func LoginWithMagicLink(ctx context.Context, email string) (*Session, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Supabase.URL == "" || cfg.Supabase.AnonKey == "" {
		return nil, errors.New("supabase project is not configured — set [supabase] in ~/.config/episki/config.toml or SUPABASE_URL/SUPABASE_ANON_KEY")
	}

	verifier, challenge, err := pkcePair()
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("open loopback listener: %w", err)
	}
	defer listener.Close()
	redirectTo := "http://" + listener.Addr().String()

	body, _ := json.Marshal(map[string]any{
		"email":                 email,
		"create_user":           false,
		"code_challenge":        challenge,
		"code_challenge_method": "s256",
	})
	resp, err := authPost(ctx, cfg.Supabase, cfg.Supabase.URL+"/auth/v1/otp?redirect_to="+url.QueryEscape(redirectTo), body)
	if err != nil {
		return nil, fmt.Errorf("request magic link: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("request magic link: %s", readAuthError(resp))
	}

	fmt.Fprintf(stderr(), "Sent a sign-in link to %s.\nClick it on this machine to finish logging in...\n", email)

	code, err := waitForLoopbackCode(ctx, listener)
	if err != nil {
		return nil, err
	}
	tok, err := exchangePKCECode(ctx, cfg.Supabase, code, verifier)
	if err != nil {
		return nil, err
	}
	session := newSessionFromToken(tok)
	if session.Email == "" {
		session.Email = email
	}
	if err := SaveSession(session); err != nil {
		return nil, err
	}
	return session, nil
}

// waitForLoopbackCode serves the loopback listener until a request carrying
// a PKCE ?code= (or an error) arrives, then returns it.
func waitForLoopbackCode(ctx context.Context, listener net.Listener) (string, error) {
	type cbResult struct {
		code string
		err  error
	}
	resultCh := make(chan cbResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errMsg := q.Get("error"); errMsg != "" {
			desc := q.Get("error_description")
			writeBrowserResponse(w, false, desc)
			resultCh <- cbResult{err: fmt.Errorf("sign-in failed: %s: %s", errMsg, desc)}
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

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resultCh:
		return res.code, res.err
	}
}

// LoginWithOTP signs in with a one-time code emailed by Supabase Auth. No
// browser or redirect allowlist involved, which makes it the most reliable
// path for terminals and headless machines.
//
// When code is "" a fresh code is requested and read from stdin; otherwise
// the given code (from a previous send) is verified directly, which is how
// non-interactive shells complete the flow.
func LoginWithOTP(ctx context.Context, email, code string) (*Session, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Supabase.URL == "" || cfg.Supabase.AnonKey == "" {
		return nil, errors.New("supabase project is not configured — set [supabase] in ~/.config/episki/config.toml or SUPABASE_URL/SUPABASE_ANON_KEY")
	}

	if code == "" {
		if err := sendOTP(ctx, cfg.Supabase, email); err != nil {
			return nil, err
		}
		fmt.Fprintf(stderr(), "Sent a sign-in code to %s. Enter it here: ", email)
		code, err = readLine(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read code: %w (in a non-interactive shell, re-run with --code <code>)", err)
		}
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("no code entered")
	}

	tok, err := verifyOTP(ctx, cfg.Supabase, email, code)
	if err != nil {
		return nil, err
	}
	session := newSessionFromToken(tok)
	if session.Email == "" {
		session.Email = email
	}
	if err := SaveSession(session); err != nil {
		return nil, err
	}
	return session, nil
}

// sendOTP asks Supabase Auth to email a one-time code. create_user=false so
// a typo'd address doesn't mint a fresh account.
func sendOTP(ctx context.Context, s config.Supabase, email string) error {
	body, _ := json.Marshal(map[string]any{
		"email":       email,
		"create_user": false,
	})
	resp, err := authPost(ctx, s, s.URL+"/auth/v1/otp", body)
	if err != nil {
		return fmt.Errorf("request sign-in code: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("request sign-in code: %s", readAuthError(resp))
	}
	return nil
}

// verifyOTP trades the emailed code for a session. The response has the same
// shape as the PKCE/refresh token grants.
func verifyOTP(ctx context.Context, s config.Supabase, email, code string) (*supabaseTokenResp, error) {
	body, _ := json.Marshal(map[string]string{
		"type":  "email",
		"email": email,
		"token": code,
	})
	resp, err := authPost(ctx, s, s.URL+"/auth/v1/verify", body)
	if err != nil {
		return nil, fmt.Errorf("verify code: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("verify code: %s", readAuthError(resp))
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

func authPost(ctx context.Context, s config.Supabase, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", s.AnonKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	return client.Do(req)
}

// readAuthError extracts the human-readable message from a GoTrue error body.
func readAuthError(resp *http.Response) string {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	var e struct {
		Msg     string `json:"msg"`
		Message string `json:"message"`
		Error   string `json:"error_description"`
	}
	if json.Unmarshal(raw, &e) == nil {
		for _, m := range []string{e.Msg, e.Message, e.Error} {
			if m != "" {
				return fmt.Sprintf("%s: %s", resp.Status, m)
			}
		}
	}
	return fmt.Sprintf("%s: %s", resp.Status, strings.TrimSpace(string(raw)))
}

func readLine(r io.Reader) (string, error) {
	return bufio.NewReader(r).ReadString('\n')
}
