package main

import (
	"github.com/episki/episki-cli/internal/auth"
	"github.com/spf13/cobra"
)

// addResourceCommands wires the data-API subcommands (`episki users`,
// `episki customers`, `episki organizations`, ...). Each subcommand uses
// internal/api to obtain a postgrest-go client tied to the user's JWT.
//
// Empty for now — add resources here as we build them. Keeping the wiring in
// its own file means /cmd/episki/main.go stays stable and the addition is a
// one-line edit.
func addResourceCommands(_ *cobra.Command, _ *auth.RootFlags) {}
