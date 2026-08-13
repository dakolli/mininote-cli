package cmd

import (
	"errors"
	"fmt"
	"os"
)

// Execute runs the mininote CLI root command and exits non-zero on failure.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// rpcErr already reported *client.APIError failures; anything else is
		// printed here once.
		if !errors.Is(err, errExit) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}
