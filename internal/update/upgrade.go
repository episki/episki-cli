package update

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// Command returns the `episki upgrade` command.
func Command(currentVersion string) *cobra.Command {
	var (
		version string
		force   bool
	)
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Reinstall episki, optionally pinning a version",
		Long: `Re-runs the official install script to upgrade (or reinstall) episki.

By default this fetches the latest release. Use --version to pin a specific
release, or --force to reinstall the version you already have.

Set EPISKI_INSTALL_DIR to override where the binary is placed.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			script, err := exec.LookPath("sh")
			if err != nil {
				return fmt.Errorf("sh not found on PATH: %w", err)
			}
			line := "curl -sSf https://cli.episki.com/install.sh | sh -s --"
			if version != "" {
				line += " --version " + version
			}
			if force {
				line += " --force"
			}
			args := []string{"-c", line}
			c := exec.Command(script, args...)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Env = os.Environ()
			fmt.Fprintf(os.Stderr, "Running: %s %s\n", script, args[1])
			return c.Run()
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Pin a specific version (e.g. 0.3.1)")
	cmd.Flags().BoolVar(&force, "force", false, "Reinstall even if already at the target version")
	return cmd
}
