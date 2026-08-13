package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var setTokenCmd = &cobra.Command{
	Use:   "set-token <token>",
	Short: "Save an API key or session token to the config file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := Load(flagConfig)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		cfg.Token = args[0]
		if err := Save(flagConfig, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Token saved to %s\n", flagConfig)
		return nil
	},
}

var showPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the config file path",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), flagConfig)
		if _, err := os.Stat(flagConfig); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "(file does not exist yet)")
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(setTokenCmd)
	configCmd.AddCommand(showPathCmd)
}
