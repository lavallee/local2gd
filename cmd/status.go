package cmd

import (
	"fmt"

	"github.com/lavallee/local2gd/internal/auth"
	"github.com/lavallee/local2gd/internal/gdrive"
	"github.com/lavallee/local2gd/internal/sync"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [pairing]",
	Short: "Show sync status without making changes",
	Long:  "Scan local and remote folders and show what would change on next sync.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		verbose, _ := cmd.Flags().GetBool("verbose")

		if !auth.HasToken() {
			return fmt.Errorf("not authenticated — run `local2gd auth` first")
		}

		configs, err := sync.LoadConfig()
		if err != nil {
			return err
		}

		httpClient, err := auth.Client(ctx)
		if err != nil {
			return fmt.Errorf("failed to get authorized client: %w", err)
		}

		client, err := gdrive.NewClient(ctx, httpClient)
		if err != nil {
			return fmt.Errorf("failed to create Drive client: %w", err)
		}

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

		for _, pairing := range pairings {
			if err := showStatus(client, pairing, verbose); err != nil {
				fmt.Printf("Error checking '%s': %v\n", pairing.Name, err)
			}
		}

		return nil
	},
}

func showStatus(client *gdrive.Client, pairing sync.PairingConfig, verbose bool) error {
	fmt.Printf("%s (%s ↔ %s)\n", pairing.Name, pairing.LocalDir, pairing.RemotePath)

	localDir, err := sync.ExpandPath(pairing.LocalDir)
	if err != nil {
		return err
	}

	state, err := sync.LoadState(localDir)
	if err != nil {
		return err
	}

	folderID := state.RemoteFolderID
	if folderID == "" {
		folderID, err = client.ResolvePath(pairing.RemotePath)
		if err != nil {
			return fmt.Errorf("remote folder not found: %w", err)
		}
	}

	localFiles, err := sync.ScanLocal(localDir)
	if err != nil {
		return err
	}

	remoteFiles, err := sync.ScanRemote(client, folderID)
	if err != nil {
		return err
	}

	actions := sync.ClassifyActions(localFiles, remoteFiles, state)

	// Count by type
	counts := make(map[sync.ActionType]int)
	var actionFiles map[sync.ActionType][]string
	if verbose {
		actionFiles = make(map[sync.ActionType][]string)
	}
	for _, a := range actions {
		counts[a.Type]++
		if verbose && a.Type != sync.ActionUnchanged {
			actionFiles[a.Type] = append(actionFiles[a.Type], a.RelPath)
		}
	}

	printCount := func(label string, t sync.ActionType) {
		if c := counts[t]; c > 0 {
			fmt.Printf("  %-20s %d file(s)\n", label+":", c)
			if verbose {
				for _, f := range actionFiles[t] {
					fmt.Printf("    %s\n", f)
				}
			}
		}
	}

	printCount("New locally", sync.ActionCreateRemote)
	printCount("New remotely", sync.ActionCreateLocal)
	printCount("Modified locally", sync.ActionPush)
	printCount("Modified remotely", sync.ActionPull)
	printCount("Deleted locally", sync.ActionDeleteRemote)
	printCount("Deleted remotely", sync.ActionDeleteLocal)
	printCount("Conflicts", sync.ActionConflict)

	unchanged := counts[sync.ActionUnchanged]
	fmt.Printf("  %-20s %d file(s)\n", "Unchanged:", unchanged)

	fmt.Println()
	return nil
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
