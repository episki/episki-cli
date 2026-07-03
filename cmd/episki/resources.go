package main

import (
	"github.com/episki/episki-cli/internal/auth"
	"github.com/episki/episki-cli/internal/resources"
	"github.com/spf13/cobra"
)

// addResourceCommands wires the data-API subcommands (`episki workspaces`,
// `episki controls`, ...). Each subcommand uses internal/api to obtain a
// postgrest-go client tied to the user's JWT; RLS in Postgres is the
// authorization boundary.
func addResourceCommands(root *cobra.Command, rf *auth.RootFlags) {
	resources.Register(root, rf)
}
