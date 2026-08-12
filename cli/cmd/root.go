package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mininote.dev/cli/client"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "mininote",
		Short: "mininote CLI",
		Long:  "mininote CLI talks to a mininote server over its typed RPC API.",
		// Errors are reported by the commands themselves (see rpcErr and
		// Execute); cobra should not double-print or dump usage on failure.
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	flagBaseURL string
	flagConfig  string
	flagToken   string
	flagCompact bool
)

//go:generate go run mininote.dev/cli/cmd/cmdgen -introspect https://mininote.ink/rpc/_introspect -forbidden ../../api-key-forbidden.txt

func init() {
	pf := rootCmd.PersistentFlags()
	defaultBaseURL := os.Getenv("MININOTE_BASE_URL")
	if defaultBaseURL == "" {
		defaultBaseURL = "https://mininote.ink"
	}
	pf.StringVar(&flagBaseURL, "base-url", defaultBaseURL, "mininote server base URL")
	pf.StringVar(&flagConfig, "config", defaultConfigPath(), "path to the CLI config file")
	pf.StringVar(&flagToken, "token", "", "override the stored auth token")
	pf.BoolVar(&flagCompact, "compact", false, "single-line JSON output")

	registerServiceCommands(rootCmd, getClient)
	rootCmd.AddCommand(versionCmd, logoutCmd)
}

// getClient resolves the base URL and token from flags, config, and the
// MININOTE_RPC_KEY / MININOTE_TOKEN environment variables, then builds a fresh
// client for the current invocation. Keys (mnk_...) are treated as API keys and
// therefore cannot call session-only control-plane RPCs.
func getClient() (*client.Client, error) {
	baseURL, token, err := resolveEndpoint()
	if err != nil {
		return nil, err
	}
	if baseURL == "" {
		return nil, errors.New("base url is empty; set --base-url or MININOTE_BASE_URL")
	}
	if strings.HasPrefix(token, "mnk_") {
		return client.New(baseURL, client.WithAPIKey(token))
	}
	return client.New(baseURL, client.WithToken(token))
}

// resolveEndpoint merges --base-url/--token with the config file and env. An
// explicit flag wins; otherwise the config file value, otherwise the
// environment (MININOTE_BASE_URL / MININOTE_RPC_KEY / MININOTE_TOKEN), for the
// base URL falling back to https://mininote.ink.
func resolveEndpoint() (baseURL, token string, err error) {
	cfg, err := Load(flagConfig)
	if err != nil {
		return "", "", err
	}
	baseURL = flagBaseURL
	if !rootCmd.PersistentFlags().Changed("base-url") && cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}
	token = flagToken
	if !rootCmd.PersistentFlags().Changed("token") {
		token = cfg.Token
	}
	if token == "" {
		token = os.Getenv("MININOTE_RPC_KEY")
	}
	if token == "" {
		token = os.Getenv("MININOTE_TOKEN")
	}
	return baseURL, token, nil
}

// defaultConfigPath returns $XDG_CONFIG_HOME/mininote/cli.json, falling back
// to ~/.config/mininote/cli.json.
func defaultConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mininote", "cli.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "mininote", "cli.json")
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
		fmt.Fprintln(cmd.OutOrStdout(), "mininote-cli dev")
		return nil
	},
}
