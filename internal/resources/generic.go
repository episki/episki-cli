package resources

import (
	"fmt"

	"github.com/episki/episki-cli/internal/auth"
	"github.com/spf13/cobra"
	"github.com/supabase-community/postgrest-go"
)

// resourceSpec describes a workspace-scoped, soft-deleted table with the
// standard list/get shape shared by most entities.
type resourceSpec struct {
	use        string // command name, e.g. "policies"
	aliases    []string
	short      string
	table      string
	listSelect string
	getSelect  string
	refCol     string // non-id lookup column for `get` ("" = id only)
	cols       []column
}

func (s resourceSpec) command(rf *auth.RootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     s.use,
		Aliases: s.aliases,
		Short:   s.short,
	}
	cmd.AddCommand(s.listCmd(rf))
	cmd.AddCommand(s.getCmd(rf))
	return cmd
}

func (s resourceSpec) listCmd(rf *auth.RootFlags) *cobra.Command {
	var lf listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List " + s.use + " in the active workspace",
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
			raw, _, err := c.client.From(s.table).
				Select(s.listSelect, "", false).
				Eq("workspace_id", ws).
				Is("deleted_at", "null").
				Order("updated_at", &postgrest.OrderOpts{Ascending: false}).
				Limit(lf.limit, "").
				Execute()
			if err != nil {
				return err
			}
			return printList(raw, s.cols, lf.jsonOut)
		},
	}
	addListFlags(cmd, &lf)
	return cmd
}

func (s resourceSpec) getCmd(rf *auth.RootFlags) *cobra.Command {
	use := "get <id>"
	if s.refCol != "" {
		use = "get <id|" + s.refCol + ">"
	}
	return &cobra.Command{
		Use:   use,
		Short: "Show one of the " + s.use + " as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := connect(cmd.Context(), rf)
			if err != nil {
				return err
			}
			ws, err := c.requireWorkspace()
			if err != nil {
				return err
			}
			col := "id"
			if !isUUID(args[0]) {
				if s.refCol == "" {
					return fmt.Errorf("%q is not a UUID", args[0])
				}
				col = s.refCol
			}
			raw, _, err := c.client.From(s.table).
				Select(s.getSelect, "", false).
				Eq("workspace_id", ws).
				Is("deleted_at", "null").
				Eq(col, args[0]).
				Single().
				Execute()
			if err != nil {
				return err
			}
			return printJSON(raw)
		},
	}
}
