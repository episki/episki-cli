package auth

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
	"github.com/spf13/cobra"
)

// RootFlags is what cmd/episki/main.go fills in before calling Command(). We
// keep it tiny on purpose — the resolver reads --api-key, everything else is
// re-loaded from config.
type RootFlags struct {
	APIKey string
	Debug  bool
}

// Command returns the `episki auth` subtree.
func Command(rf *RootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}
	cmd.AddCommand(loginCmd())
	cmd.AddCommand(logoutCmd())
	cmd.AddCommand(statusCmd(rf))
	cmd.AddCommand(whoamiCmd(rf))
	cmd.AddCommand(refreshCmd())
	return cmd
}

// refreshCmd forces a token refresh even when the access token hasn't
// expired. Workspace claims are stamped server-side (by `episki workspaces
// use` or the web app) and only reach this CLI's JWT on the next refresh, so
// this is the way to pick up an out-of-band workspace switch.
func refreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the stored session token (picks up workspace changes)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := LoadSession()
			if err != nil {
				return err
			}
			if s == nil || s.RefreshToken == "" {
				return fmt.Errorf("no stored session — run `episki auth login`")
			}
			refreshed, err := RefreshSession(cmd.Context(), s)
			if err != nil {
				return err
			}
			if ws := WorkspaceID(refreshed.AccessToken); ws != "" {
				fmt.Fprintf(os.Stderr, "Session refreshed. Active workspace: %s\n", ws)
			} else {
				fmt.Fprintln(os.Stderr, "Session refreshed. No workspace selected — run `episki workspaces use <id|slug>`.")
			}
			return nil
		},
	}
}

func loginCmd() *cobra.Command {
	var provider, email, code string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in via your browser, or with an emailed code (--email)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			var (
				s   *Session
				err error
			)
			if code != "" && email == "" {
				return fmt.Errorf("--code requires --email")
			}
			switch {
			case code != "":
				s, err = LoginWithOTP(ctx, email, code)
			case email != "":
				s, err = LoginWithMagicLink(ctx, email)
			default:
				s, err = Login(ctx, LoginOptions{Provider: provider})
			}
			if err != nil {
				return err
			}
			who := s.Email
			if who == "" {
				who = "session active"
			}
			fmt.Fprintf(os.Stderr, "Logged in as %s\n", who)
			return nil
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "OAuth provider routed through Supabase Auth (e.g. google, github)")
	cmd.Flags().StringVar(&email, "email", "", "Sign in via a magic link emailed to this address instead of the browser OAuth flow")
	cmd.Flags().StringVar(&code, "code", "", "Verify an emailed one-time code non-interactively (requires --email)")
	return cmd
}

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored session from your keychain",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := DeleteSession(); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Logged out.")
			return nil
		},
	}
}

func statusCmd(rf *RootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show which credential is active",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunStatus(cmd.Context(), rf)
		},
	}
}

// RunStatus prints the active credential summary. Exposed so cmd/episki can
// register `episki status` as a top-level alias of `episki auth status`.
func RunStatus(ctx context.Context, rf *RootFlags) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cred, err := Resolve(ctx, rf.APIKey)
	if err != nil && cred.Kind == CredNone {
		fmt.Fprintf(os.Stdout, "Not authenticated.\n  Project: %s\n", cfg.Supabase.URL)
		fmt.Fprintln(os.Stdout, "  Run `episki auth login` or set EPISKI_API_KEY.")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Authenticated\n")
	fmt.Fprintf(os.Stdout, "  Source:  %s\n", cred.Kind)
	if cred.Session != nil && cred.Session.Email != "" {
		fmt.Fprintf(os.Stdout, "  User:    %s\n", cred.Session.Email)
	}
	fmt.Fprintf(os.Stdout, "  Project: %s\n", cfg.Supabase.URL)
	if ws := WorkspaceID(cred.Token); ws != "" {
		label := ws
		if cfg.Workspace.ID == ws && cfg.Workspace.Name != "" {
			label = fmt.Sprintf("%s (%s)", cfg.Workspace.Name, ws)
		}
		fmt.Fprintf(os.Stdout, "  Workspace: %s\n", label)
	} else {
		fmt.Fprintln(os.Stdout, "  Workspace: none — run `episki workspaces use <id|slug>`")
	}
	return nil
}

// whoamiCmd is a thin call against Supabase Auth's /auth/v1/user. It works
// for both API-key and OAuth-session credentials and is the easiest way to
// smoke-test that auth is wired correctly.
func whoamiCmd(rf *RootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print details for the current credential",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cred, err := Resolve(cmd.Context(), rf.APIKey)
			if err != nil {
				return err
			}

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, cfg.Supabase.UserURL(), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+cred.Token)
			req.Header.Set("apikey", cfg.Supabase.AnonKey)
			req.Header.Set("Accept", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode/100 != 2 {
				return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
			}
			var out any
			if err := json.Unmarshal(body, &out); err != nil {
				_, _ = os.Stdout.Write(body)
				return nil
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
}

func stderr() io.Writer { return os.Stderr }
