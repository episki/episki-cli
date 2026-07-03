package resources

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/episki/episki-cli/internal/appapi"
	"github.com/episki/episki-cli/internal/auth"
	"github.com/episki/episki-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/supabase-community/postgrest-go"
)

// The workspaces table is deliberately not scoped by the JWT workspace
// claim — RLS grants SELECT on any workspace the user is a member of — so
// these commands work even before a workspace is selected.

func workspacesCmd(rf *auth.RootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspaces",
		Aliases: []string{"ws"},
		Short:   "List and switch workspaces",
	}
	cmd.AddCommand(workspacesListCmd(rf))
	cmd.AddCommand(workspacesCurrentCmd(rf))
	cmd.AddCommand(workspacesUseCmd(rf))
	return cmd
}

const workspaceSelect = "id,name,slug,billing_plan,workspace_kind,created_at"

func workspacesListCmd(rf *auth.RootFlags) *cobra.Command {
	var lf listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspaces you are a member of",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := connect(cmd.Context(), rf)
			if err != nil {
				return err
			}
			raw, _, err := c.client.From("workspaces").
				Select(workspaceSelect, "", false).
				Order("created_at", &postgrest.OrderOpts{Ascending: true}).
				Limit(lf.limit, "").
				Execute()
			if err != nil {
				return err
			}
			if lf.jsonOut {
				return printJSON(raw)
			}
			// Mark the workspace the JWT claim points at.
			var rows []map[string]any
			if err := json.Unmarshal(raw, &rows); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			active := c.workspaceID()
			for _, row := range rows {
				if id, _ := row["id"].(string); id == active {
					row["active"] = "*"
				}
			}
			marked, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printList(marked, []column{
				{"", "active"}, {"id", "id"}, {"name", "name"}, {"slug", "slug"},
				{"plan", "billing_plan"}, {"kind", "workspace_kind"}, {"created", "created_at"},
			}, false)
		},
	}
	addListFlags(cmd, &lf)
	return cmd
}

func workspacesCurrentCmd(rf *auth.RootFlags) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Show the active workspace (from the JWT claim)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := connect(cmd.Context(), rf)
			if err != nil {
				return err
			}
			ws, err := c.requireWorkspace()
			if err != nil {
				return err
			}
			raw, _, err := c.client.From("workspaces").
				Select(workspaceSelect, "", false).
				Eq("id", ws).
				Single().
				Execute()
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(raw)
			}
			var row map[string]any
			if err := json.Unmarshal(raw, &row); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			fmt.Fprintf(os.Stdout, "%s (%s)\n", field(row, "name"), field(row, "id"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit the raw JSON response")
	return cmd
}

func workspacesUseCmd(rf *auth.RootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use <id|slug>",
		Short: "Switch the active workspace",
		Long: "Switches the active workspace by asking the episki app to stamp the\n" +
			"workspace claim onto your auth user, then refreshing the local session\n" +
			"so the new JWT carries it. All workspace-scoped commands follow the claim.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := connect(ctx, rf)
			if err != nil {
				return err
			}
			if c.cred.Kind != auth.CredOAuthSession || c.cred.Session == nil {
				return errors.New("workspaces use needs an OAuth session (run `episki auth login`) — API-key credentials can't switch workspaces")
			}

			// Resolve slug or id against the membership-scoped workspaces table.
			col := "slug"
			if isUUID(args[0]) {
				col = "id"
			}
			raw, _, err := c.client.From("workspaces").
				Select("id,name,slug", "", false).
				Eq(col, args[0]).
				Single().
				Execute()
			if err != nil {
				return fmt.Errorf("workspace %q not found (are you a member?): %w", args[0], err)
			}
			var target struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(raw, &target); err != nil {
				return fmt.Errorf("decode workspace: %w", err)
			}

			if err := appapi.SwitchWorkspace(ctx, c.cfg.AppURL, c.cred.Token, target.ID); err != nil {
				return fmt.Errorf("%w\nFallback: switch workspaces at %s, then run `episki auth refresh`", err, c.cfg.AppURL)
			}

			refreshed, err := auth.RefreshSession(ctx, c.cred.Session)
			if err != nil {
				return fmt.Errorf("workspace switched but token refresh failed: %w\nRun `episki auth refresh` to finish", err)
			}
			if got := auth.WorkspaceID(refreshed.AccessToken); got != target.ID {
				return fmt.Errorf("workspace claim did not stick (token carries %q) — check your membership and try again", got)
			}

			// Display-only bookkeeping; the JWT claim stays the authority.
			cfg := c.cfg
			cfg.Workspace = config.Workspace{ID: target.ID, Name: target.Name}
			if err := config.Save(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", err)
			}

			fmt.Fprintf(os.Stderr, "Active workspace: %s (%s)\n", target.Name, target.ID)
			return nil
		},
	}
}
