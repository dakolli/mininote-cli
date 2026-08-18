package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/dakolli/mininote-cli/cmd/store"
	"github.com/spf13/cobra"
)

var (
	flagAddKeyName      string
	flagAddKeyWorkspace string
	flagAddKeyType      string
)

var keyCmd = &cobra.Command{
	Use:     "key",
	Aliases: []string{"keys"},
	GroupID: "management",
	Short:   "Manage stored API and MCP credentials in the key vault",
	Long: `Manage credentials stored in ~/.config/mininote/mininote.db.

Available subcommands:
  add       Save a named token to the vault (flags: --name, --type rpc|mcp|multi, --workspace)
  list, ls  List all stored keys with masked tokens, type, and active status
  rm        Remove a stored key by name
  use       Set the active key by name
  current   Show the currently active key
  path      Print the database path`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runListKeys(cmd)
	},
}

var keyAddCmd = &cobra.Command{
	Use:     "add <token>",
	Aliases: []string{"set", "create"},
	Short:   "Save a named API or MCP key to the database",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagAddKeyName == "" {
			return errors.New("--name is required")
		}
		kt := store.KeyType(flagAddKeyType)
		if !kt.IsValid() {
			return fmt.Errorf("invalid type %q (must be 'rpc', 'mcp', or 'multi')", flagAddKeyType)
		}
		st, err := store.GetStore("")
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()

		rec := store.KeysRecord{
			Name:      flagAddKeyName,
			Workspace: flagAddKeyWorkspace,
			Token:     args[0],
			Type:      kt,
		}
		if err := st.PutKey(rec); err != nil {
			return fmt.Errorf("save key: %w", err)
		}

		active, _ := st.GetActiveKey()
		if active == "" {
			_ = st.SetActiveKey(rec.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Key %q (%s) saved and set as active.\n", rec.Name, rec.Type)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Key %q (%s) saved.\n", rec.Name, rec.Type)
		}
		return nil
	},
}

var keyListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List all stored API and MCP keys",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runListKeys(cmd)
	},
}

func runListKeys(cmd *cobra.Command) error {
	st, err := store.GetStore("")
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	keys, err := st.Keys()
	if err != nil {
		return fmt.Errorf("list keys: %w", err)
	}
	if len(keys) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No keys stored. Add one with: mininote key add <token> --name <name>")
		return nil
	}

	active, _ := st.GetActiveKey()
	for _, k := range keys {
		marker := " "
		if k.Name == active {
			marker = "*"
		}
		masked := k.Token
		if len(masked) > 8 {
			masked = masked[:4] + "..." + masked[len(masked)-4:]
		}
		ws := k.Workspace
		if ws == "" {
			ws = "-"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s [type: %s, workspace: %s, token: %s]\n", marker, k.Name, k.Type, ws, masked)
	}
	return nil
}

var keyRmCmd = &cobra.Command{
	Use:     "rm <name>",
	Aliases: []string{"remove", "delete", "del"},
	Short:   "Delete a stored key by name",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := store.GetStore("")
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()

		if err := st.DeleteKey(args[0]); err != nil {
			return fmt.Errorf("delete key: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Key %q deleted.\n", args[0])
		return nil
	},
}

var keyUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the active key by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		st, err := store.GetStore("")
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()

		rec, err := st.GetKey(name)
		if err != nil {
			return fmt.Errorf("lookup key: %w", err)
		}
		if rec == nil {
			return fmt.Errorf("key %q not found in store", name)
		}

		if err := st.SetActiveKey(name); err != nil {
			return fmt.Errorf("set active key: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Active key set to %q.\n", name)
		return nil
	},
}

var keyCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the currently active key",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := store.GetStore("")
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()

		active, err := st.GetActiveKey()
		if err != nil {
			return fmt.Errorf("get active key: %w", err)
		}
		if active == "" {
			keys, _ := st.Keys()
			if len(keys) == 1 {
				fmt.Fprintf(cmd.OutOrStdout(), "No active key explicitly set. Defaulting to single stored key: %q\n", keys[0].Name)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "No active key set.")
			return nil
		}

		rec, err := st.GetKey(active)
		if err != nil || rec == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Active key: %s (record missing from store)\n", active)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Active key: %s [type: %s, workspace: %s]\n", rec.Name, rec.Type, rec.Workspace)
		return nil
	},
}

var keyPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the database path",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath := store.DefaultPath()
		fmt.Fprintf(cmd.OutOrStdout(), "Store: %s\n", dbPath)
		if _, err := os.Stat(dbPath); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "  (database does not exist yet)")
		}
		return nil
	},
}

func init() {
	keyAddCmd.Flags().StringVarP(&flagAddKeyName, "name", "n", "", "key name or alias (required)")
	keyAddCmd.Flags().StringVar(&flagAddKeyType, "type", "rpc", "key type: 'rpc', 'mcp', or 'multi'")
	keyAddCmd.Flags().StringVarP(&flagAddKeyWorkspace, "workspace", "w", "", "workspace identifier (optional)")

	keyCmd.AddCommand(keyAddCmd)
	keyCmd.AddCommand(keyListCmd)
	keyCmd.AddCommand(keyRmCmd)
	keyCmd.AddCommand(keyUseCmd)
	keyCmd.AddCommand(keyCurrentCmd)
	keyCmd.AddCommand(keyPathCmd)

	rootCmd.AddCommand(keyCmd)
}
