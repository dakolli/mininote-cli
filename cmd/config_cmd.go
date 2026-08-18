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

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration and keys",
}

var addTokenCmd = &cobra.Command{
	Use:   "add-token <token>",
	Short: "Save a named API or MCP key to the database",
	Args:  cobra.ExactArgs(1),
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

var listTokensCmd = &cobra.Command{
	Use:   "list-tokens",
	Short: "List all stored API and MCP keys",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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
			fmt.Fprintln(cmd.OutOrStdout(), "No keys stored.")
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
	},
}

var useTokenCmd = &cobra.Command{
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

var currentTokenCmd = &cobra.Command{
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
			// check if single key fallback applies
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

var deleteTokenCmd = &cobra.Command{
	Use:   "delete-token <name>",
	Short: "Delete a stored key by name",
	Args:  cobra.ExactArgs(1),
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

var showPathCmd = &cobra.Command{
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
	addTokenCmd.Flags().StringVarP(&flagAddKeyName, "name", "n", "", "key name or alias (required)")
	addTokenCmd.Flags().StringVar(&flagAddKeyType, "type", "rpc", "key type: 'rpc', 'mcp', or 'multi'")
	addTokenCmd.Flags().StringVarP(&flagAddKeyWorkspace, "workspace", "w", "", "workspace identifier (optional)")

	configCmd.AddCommand(addTokenCmd)
	configCmd.AddCommand(listTokensCmd)
	configCmd.AddCommand(useTokenCmd)
	configCmd.AddCommand(currentTokenCmd)
	configCmd.AddCommand(deleteTokenCmd)
	configCmd.AddCommand(showPathCmd)
}
