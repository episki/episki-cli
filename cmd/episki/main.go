// Command episki is the official episki command-line interface.
//
// All API calls go through the Supabase Data API (PostgREST) using the
// signed-in user's JWT, so RLS policies in Postgres are the authorization
// boundary — not code in this binary.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/episki/episki-cli/internal/auth"
	"github.com/episki/episki-cli/internal/update"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rf := &auth.RootFlags{}

	root := &cobra.Command{
		Use:           "episki",
		Short:         "The official command-line interface for episki",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}
	root.PersistentFlags().StringVar(&rf.APIKey, "api-key", "", "Supabase user JWT for non-interactive use")
	root.PersistentFlags().BoolVar(&rf.Debug, "debug", false, "Enable debug logging (HTTP requests/responses)")

	root.AddCommand(auth.Command(rf))
	root.AddCommand(update.Command(Version))

	// `episki status` as a top-level alias for `episki auth status` — matches
	// Mercury's UX.
	root.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show which credential is active (alias for `episki auth status`)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return auth.RunStatus(cmd.Context(), rf)
		},
	})

	addResourceCommands(root, rf)

	checkDone := make(chan struct{})
	go func() {
		defer close(checkDone)
		update.CheckOnce(ctx, Version)
	}()

	err := root.ExecuteContext(ctx)
	<-checkDone

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
