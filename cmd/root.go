package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dakolli/mininote-cli/client"
	"github.com/dakolli/mininote-cli/cmd/store"

	"github.com/spf13/cobra"
)

const defaultBaseURL = "https://mininote.ink"

var (
	rootCmd = &cobra.Command{
		Use:   "mininote",
		Short: "mininote CLI",
		Long:  "mininote CLI talks to a mininote server over its typed RPC API.",
		// Errors are reported by the commands themselves (see rpcErr and
		// Execute); cobra should not double-print or dump usage on failure.
		SilenceErrors: true,
		SilenceUsage:  true,
		// The CLI is a 1:1 RPC surface; cobra's auto-generated `completion`
		// command is not part of it.
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}

	flagKeyName string
	flagToken   string
	flagCompact bool
)

//go:generate go run github.com/dakolli/mininote-cli/cmd/cmdgen -introspect https://mininote.ink/rpc/_introspect -forbidden ../api-key-forbidden.txt

func init() {
	rootCmd.AddGroup(&cobra.Group{
		ID:    "services",
		Title: "Mininote API:",
	})
	rootCmd.AddGroup(&cobra.Group{
		ID:    "management",
		Title: "Management & Integrations:",
	})

	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&flagKeyName, "key", "k", "", "named key to authenticate with from the store")
	pf.StringVarP(&flagToken, "token", "t", "", "override with an explicit auth token")
	pf.BoolVar(&flagCompact, "compact", false, "single-line JSON output")

	registerServiceCommands(rootCmd, getClient)
	rootCmd.AddCommand(versionCmd)
}

// getClient resolves the auth token and builds a fresh client for the current
// invocation. Keys (mnk_...) are treated as API keys and therefore cannot call
// session-only control-plane RPCs.
func getClient() (*client.Client, error) {
	token, err := resolveToken()
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(token, "mnk_") {
		return client.New(defaultBaseURL, client.WithAPIKey(token))
	}
	return client.New(defaultBaseURL, client.WithToken(token))
}

// resolveToken resolves the token for RPC commands from flags, env, or store.
func resolveToken() (string, error) {
	return resolveTokenFor(store.KeyTypeRPC)
}

// resolveTokenFor resolves the token for a specific purpose (RPC or MCP) from
// an explicit flag, environment variables, or the bbolt key store.
func resolveTokenFor(purpose store.KeyType) (string, error) {
	if flagToken != "" {
		return flagToken, nil
	}
	if env := os.Getenv("MININOTE_RPC_KEY"); env != "" {
		return env, nil
	}
	if env := os.Getenv("MININOTE_TOKEN"); env != "" {
		return env, nil
	}

	st, err := store.GetStore("")
	if err != nil {
		return "", fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	rec, err := st.ResolveKeyFor(flagKeyName, purpose)
	if err != nil {
		return "", err
	}
	return rec.Token, nil
}

// errExit is the sentinel returned once rpcErr has already reported the error,
// so the command exits non-zero without cobra printing anything extra.
var errExit = errors.New("mininote: command failed")

// rpcErr normalizes a client call error for the generated commands. An
// *client.APIError is printed as a single friendly Error: line and replaced by
// errExit; any other error is returned wrapped for Execute to report.
func rpcErr(err error) error {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		msg := apiErr.Message
		if msg == "" {
			msg = strings.TrimSpace(apiErr.Body)
		}
		if apiErr.StatusCode > 0 {
			msg = fmt.Sprintf("%s (status %d)", msg, apiErr.StatusCode)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
		return errExit
	}
	return fmt.Errorf("mininote: %w", err)
}

// printResult prints v as indented JSON, or as a single line with --compact.
func printResult(cmd *cobra.Command, v any) error {
	compact, _ := cmd.Root().PersistentFlags().GetBool("compact")
	var out []byte
	var err error
	if compact {
		out, err = json.Marshal(v)
	} else {
		out, err = json.MarshalIndent(v, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the mininote CLI version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "mininote-cli "+version)
		return nil
	},
}
