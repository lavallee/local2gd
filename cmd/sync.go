package cmd

import (
	"fmt"

	"github.com/lavallee/local2gd/internal/auth"
	"github.com/lavallee/local2gd/internal/gdrive"
	"github.com/lavallee/local2gd/internal/sync"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [pairing]",
	Short: "Sync local markdown files with Google Drive",
	Long:  "Scan local and remote folders, detect changes, and sync files bidirectionally.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		noDelete, _ := cmd.Flags().GetBool("no-delete")

		// Check auth
		if !auth.HasToken() {
			return fmt.Errorf("not authenticated — run `local2gd auth` first")
		}

		// Load config
		configs, err := sync.LoadConfig()
		if err != nil {
			return err
		}

		// Get authorized client
		httpClient, err := auth.Client(ctx)
		if err != nil {
			return fmt.Errorf("failed to get authorized client: %w", err)
		}

		// Create Drive client
		client, err := gdrive.NewClient(ctx, httpClient)
		if err != nil {
			return fmt.Errorf("failed to create Drive client: %w", err)
		}

		// Determine which pairings to sync
		var pairings []sync.PairingConfig
		if len(args) > 0 {
			p, err := sync.FindPairing(configs, args[0])
			if err != nil {
				return err
			}
			pairings = []sync.PairingConfig{*p}
		} else {
			pairings = configs
		}

		// Sync each pairing
		for i, pairing := range pairings {
			if len(pairings) > 1 {
				if i > 0 {
					fmt.Println()
				}
				fmt.Printf("=== Syncing: %s (%s ↔ %s) ===\n", pairing.Name, pairing.LocalDir, pairing.RemotePath)
			}

			engine := sync.NewEngine(client, pairing)
			engine.SetNoDelete(noDelete)

			report, err := engine.Run(dryRun)
			if err != nil {
				fmt.Printf("Error syncing '%s': %v\n", pairing.Name, err)
				continue
			}

			if len(report.Errors) > 0 {
				fmt.Printf("Completed with %d error(s)\n", len(report.Errors))
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().Bool("dry-run", false, "Preview changes without syncing")
	syncCmd.Flags().Bool("no-delete", false, "Skip deletion propagation")
}
