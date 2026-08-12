package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// logoutCmd clears the stored token locally.
//
// There is no hand-written `mininote login` or `mininote whoami`: Auth RPCs are
// session-only and redacted from the live introspect route, and the generated
// client is strictly key-available, so no client method can reach them. logout
// only clears the config file — it makes no RPC call.
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear the stored session token",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := Load(flagConfig)
		if err != nil {
			return err
		}
		cfg.Token = ""
		cfg.RefreshToken = ""
		if err := Save(flagConfig, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "logged out")
		return nil
	},
}
