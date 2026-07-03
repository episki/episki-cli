// Package resources implements the data subcommands (`episki workspaces`,
// `episki controls`, ...). Every query goes through PostgREST with the
// signed-in user's JWT; RLS in Postgres is the authorization boundary, and
// workspace scoping comes from the JWT's app_metadata.workspace_id claim —
// the client-side workspace_id filters here are defense-in-depth only.
package resources

import (
	"github.com/episki/episki-cli/internal/auth"
	"github.com/spf13/cobra"
)

// Register attaches all resource subcommands to the root command.
func Register(root *cobra.Command, rf *auth.RootFlags) {
	root.AddCommand(workspacesCmd(rf))
	root.AddCommand(frameworksCmd(rf))
	root.AddCommand(controlsCmd(rf))
	root.AddCommand(workItemsCmd(rf))
	root.AddCommand(evidenceCmd(rf))
	root.AddCommand(policiesCmd(rf))
	root.AddCommand(risksCmd(rf))
}
