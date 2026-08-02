package cmd

// migrate_state.go - Explicit migration from the pre-v2 state namespace.

import (
	"fmt"

	"github.com/aic0d3r/z13ctl-plus/internal/daemon"
	"github.com/spf13/cobra"
)

var migrateStateCmd = &cobra.Command{
	Use:   "migrate-state",
	Short: "Copy state from the legacy z13ctl namespace",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if dryRunFlag {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Would migrate state from %s to %s\n", daemon.LegacyStatePath(), daemon.StatePath())
			return err
		}
		if err := daemon.MigrateLegacyState(); err != nil {
			return err
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Migrated state from %s to %s\n", daemon.LegacyStatePath(), daemon.StatePath())
		return err
	},
}

func init() {
	rootCmd.AddCommand(migrateStateCmd)
}
