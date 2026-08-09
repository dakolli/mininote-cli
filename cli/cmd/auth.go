package cmd

import (
	"errors"
	"fmt"

	"mininote.dev/cli/client"

	"github.com/spf13/cobra"
)

var (
	loginFlagHandle     string
	loginFlagPassword   string
	loginFlagTrustToken string
)

func init() {
	loginCmd.Flags().StringVar(&loginFlagHandle, "handle", "", "Handle")
	loginCmd.Flags().StringVar(&loginFlagPassword, "password", "", "Password")
	loginCmd.Flags().StringVar(&loginFlagTrustToken, "trust-token", "", "trusted-device token")
	_ = loginCmd.MarkFlagRequired("handle")
	_ = loginCmd.MarkFlagRequired("password")
}

// loginCmd is the hand-written top-level login (distinct from the generated
// auth login, which prints the raw RPC result). It persists the session to the
// config file so later invocations authenticate automatically.
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in and store the session token",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseURL, _, err := resolveEndpoint()
		if err != nil {
			return err
		}
		if baseURL == "" {
			return errors.New("base url is empty; set --base-url or MININOTE_BASE_URL")
		}
		// Login is unauthenticated: no token is attached to the request.
		c, err := client.New(baseURL)
		if err != nil {
			return err
		}
		req := client.AuthLoginParams{
			Handle:   loginFlagHandle,
			Password: loginFlagPassword,
		}
		if cmd.Flags().Changed("trust-token") {
			t := loginFlagTrustToken
			req.TrustToken = &t
		}
		resp, err := c.AuthLogin(cmd.Context(), req)
		if err != nil {
			return rpcErr(err)
		}

		cfg, err := Load(flagConfig)
		if err != nil {
			return err
		}
		cfg.BaseURL = baseURL
		if resp.Token != nil {
			cfg.Token = *resp.Token
		}
		if resp.RefreshToken != nil {
			cfg.RefreshToken = *resp.RefreshToken
		}
		if resp.Subject != nil {
			cfg.Subject = *resp.Subject
		}
		if resp.ExpiresAt != nil {
			cfg.ExpiresAt = *resp.ExpiresAt
		}
		if err := Save(flagConfig, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		if resp.MfaRequired != nil && *resp.MfaRequired {
			fmt.Fprintf(cmd.OutOrStdout(), "MFA required; partial credentials saved to %s\n", flagConfig)
		}
		return printResult(cmd, resp)
	},
}

// logoutCmd clears the stored token locally.
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

// whoamiCmd prints the authenticated user's identity.
var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the current authenticated user",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		resp, err := c.AuthMe(cmd.Context())
		if err != nil {
			return rpcErr(err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "handle:  %s\n", strp(resp.Handle))
		fmt.Fprintf(cmd.OutOrStdout(), "email:   %s\n", strp(resp.Email))
		fmt.Fprintf(cmd.OutOrStdout(), "subject: %s\n", strp(resp.Subject))
		return nil
	},
}

func strp(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
