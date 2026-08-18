package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dakolli/mininote-cli/cmd/store"
	"github.com/spf13/cobra"
)

const defaultMCPEndpoint = "https://mininote.ink/mcp"

var (
	flagMCPName          string
	flagMCPDryRun        bool
	flagOpenCodeLocal    bool
	flagAntigravityLocal bool
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Configure mininote MCP server in AI harnesses (claude, opencode, codex, antigravity)",
	Long: `Configure and register the mininote remote MCP server across supported AI coding harnesses.

Resolves your authentication key from the store, env, or --key/--token flags and registers
https://mininote.ink/mcp with the appropriate bearer token.`,
}

func runHarnessMCP(cmd *cobra.Command, harnessBinary string, harnessArgs []string) error {
	token, err := resolveTokenFor(store.KeyTypeMCP)
	if err != nil {
		return err
	}

	var args []string
	switch harnessBinary {
	case "claude":
		args = []string{"mcp", "add", "--transport", "http", flagMCPName, defaultMCPEndpoint, "--header", fmt.Sprintf("Authorization: Bearer %s", token)}
	case "opencode":
		if flagOpenCodeLocal {
			if flagMCPDryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would update ./opencode.json with %q remote MCP configuration.\n", flagMCPName)
				return nil
			}
			return configureOpenCodeLocal(cmd, flagMCPName, token)
		}
		args = []string{"mcp", "add", flagMCPName, "--url", defaultMCPEndpoint, "--header", fmt.Sprintf("Authorization=Bearer %s", token)}
	case "codex":
		args = []string{"mcp", "add", flagMCPName, "--url", defaultMCPEndpoint, "--bearer-token-env-var", "MININOTE_RPC_KEY"}
	case "antigravity":
		return configureAntigravityMCP(cmd, flagMCPName, token)
	default:
		args = harnessArgs
	}

	cmdStr := fmt.Sprintf("%s%s", harnessBinary, formatArgsForDisplay(args, token))

	if flagMCPDryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would execute:\n  %s\n", cmdStr)
		return nil
	}

	binPath, err := exec.LookPath(harnessBinary)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "%q CLI not found on PATH. Run this command manually to configure:\n\n  %s\n", harnessBinary, cmdStr)
		return nil
	}

	c := exec.CommandContext(cmd.Context(), binPath, args...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	c.Stdin = os.Stdin

	fmt.Fprintf(cmd.OutOrStdout(), "Adding mininote MCP to %s...\n", harnessBinary)
	if err := c.Run(); err != nil {
		return fmt.Errorf("execute %s: %w", harnessBinary, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Successfully added mininote MCP to %s.\n", harnessBinary)
	return nil
}

func configureOpenCodeLocal(cmd *cobra.Command, name, token string) error {
	filePath := "opencode.json"
	var data map[string]any

	if raw, err := os.ReadFile(filePath); err == nil {
		if err := json.Unmarshal(raw, &data); err != nil {
			return fmt.Errorf("parse existing %s: %w", filePath, err)
		}
	}
	if data == nil {
		data = make(map[string]any)
	}

	var mcpMap map[string]any
	if existingMCP, ok := data["mcp"].(map[string]any); ok {
		mcpMap = existingMCP
	} else {
		mcpMap = make(map[string]any)
	}

	mcpMap[name] = map[string]any{
		"type": "remote",
		"url":  defaultMCPEndpoint,
		"headers": map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
		},
	}
	data["mcp"] = mcpMap

	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filePath, err)
	}

	if err := os.WriteFile(filePath, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filePath, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Successfully configured MCP server %q in ./%s\n", name, filePath)
	return nil
}

func configureAntigravityMCP(cmd *cobra.Command, name, token string) error {
	var targetPath string
	if flagAntigravityLocal {
		targetPath = filepath.Join(".agents", "mcp_config.json")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve user home directory: %w", err)
		}
		targetPath = filepath.Join(home, ".gemini", "config", "mcp_config.json")
	}

	var configData map[string]any
	if raw, err := os.ReadFile(targetPath); err == nil {
		_ = json.Unmarshal(raw, &configData)
	}
	if configData == nil {
		configData = make(map[string]any)
	}

	var servers map[string]any
	if existing, ok := configData["mcpServers"].(map[string]any); ok {
		servers = existing
	} else {
		servers = make(map[string]any)
	}

	servers[name] = map[string]any{
		"serverUrl": defaultMCPEndpoint,
		"headers": map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
		},
	}
	configData["mcpServers"] = servers

	encoded, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mcp config: %w", err)
	}

	if flagMCPDryRun {
		displayConfig := map[string]any{
			"mcpServers": map[string]any{
				name: map[string]any{
					"serverUrl": defaultMCPEndpoint,
					"headers": map[string]string{
						"Authorization": fmt.Sprintf("Bearer %s", maskToken(token)),
					},
				},
			},
		}
		disp, _ := json.MarshalIndent(displayConfig, "", "  ")
		fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would update %s with:\n%s\n", targetPath, string(disp))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", filepath.Dir(targetPath), err)
	}

	if err := os.WriteFile(targetPath, append(encoded, '\n'), 0o600); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Could not write directly to %s (%v).\nAdd this configuration to your %s:\n\n%s\n",
			targetPath, err, targetPath, string(encoded))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Successfully added mininote MCP server %q to %s\n", name, targetPath)
	return nil
}

func maskToken(token string) string {
	if len(token) > 8 {
		return token[:4] + "..." + token[len(token)-4:]
	}
	return token
}

func formatArgsForDisplay(args []string, token string) string {
	maskedToken := maskToken(token)
	var out string
	for _, a := range args {
		if a == fmt.Sprintf("Authorization: Bearer %s", token) {
			out += fmt.Sprintf(" %q", fmt.Sprintf("Authorization: Bearer %s", maskedToken))
		} else if a == fmt.Sprintf("Authorization=Bearer %s", token) {
			out += fmt.Sprintf(" %q", fmt.Sprintf("Authorization=Bearer %s", maskedToken))
		} else {
			out += " " + a
		}
	}
	return out
}

var mcpClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Add mininote MCP server to Claude (Claude Code)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHarnessMCP(cmd, "claude", nil)
	},
}

var mcpOpenCodeCmd = &cobra.Command{
	Use:   "opencode",
	Short: "Add mininote MCP server to OpenCode",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHarnessMCP(cmd, "opencode", nil)
	},
}

var mcpCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Add mininote MCP server to CodeX",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHarnessMCP(cmd, "codex", nil)
	},
}

var mcpAntigravityCmd = &cobra.Command{
	Use:     "antigravity",
	Aliases: []string{"agy"},
	Short:   "Add mininote MCP server to Google Antigravity",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHarnessMCP(cmd, "antigravity", nil)
	},
}

func init() {
	mcpCmd.PersistentFlags().StringVarP(&flagMCPName, "name", "n", "mininote", "name for the MCP server registration")
	mcpCmd.PersistentFlags().BoolVar(&flagMCPDryRun, "dry-run", false, "print the command without executing")

	mcpOpenCodeCmd.Flags().BoolVarP(&flagOpenCodeLocal, "local", "l", false, "configure MCP server locally in ./opencode.json instead of globally")
	mcpAntigravityCmd.Flags().BoolVarP(&flagAntigravityLocal, "local", "l", false, "configure MCP server locally in ./.agents/mcp_config.json instead of globally")

	mcpCmd.AddCommand(mcpClaudeCmd)
	mcpCmd.AddCommand(mcpOpenCodeCmd)
	mcpCmd.AddCommand(mcpCodexCmd)
	mcpCmd.AddCommand(mcpAntigravityCmd)

	rootCmd.AddCommand(mcpCmd)
}
