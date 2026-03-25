package cmd

import (
	"fmt"

	"github.com/lavallee/local2gd/internal/auth"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Google Drive",
	Long:  "Open a browser window to authorize local2gd to access your Google Drive and Docs.",
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		if auth.HasToken() && !force {
			fmt.Println("Already authenticated. Use --force to re-authenticate.")
			return nil
		}

		token, err := auth.Login(cmd.Context())
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		if err := auth.SaveToken(token); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		fmt.Println("Authentication successful! Credentials saved.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.Flags().Bool("force", false, "Force re-authentication even if already logged in")
}
