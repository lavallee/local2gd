package cmd

import (
	"fmt"
	"os"

	"github.com/lavallee/local2gd/internal/auth"
	"github.com/lavallee/local2gd/internal/convert"
	"github.com/lavallee/local2gd/internal/gdrive"
	"github.com/lavallee/local2gd/internal/sync"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff [path]",
	Short: "Show content differences for changed files",
	Long:  "Show unified diffs between local and remote versions of changed files.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

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

		var filterPath string
		if len(args) > 0 {
			filterPath = args[0]
		}

		for _, pairing := range configs {
			if err := showDiff(client, pairing, filterPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error diffing '%s': %v\n", pairing.Name, err)
			}
		}

		return nil
	},
}

func showDiff(client *gdrive.Client, pairing sync.PairingConfig, filterPath string) error {
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

	dmp := diffmatchpatch.New()
	shown := 0

	for _, a := range actions {
		if a.Type == sync.ActionUnchanged {
			continue
		}

		if filterPath != "" && a.RelPath != filterPath {
			continue
		}

		switch a.Type {
		case sync.ActionPush, sync.ActionPull, sync.ActionConflict:
			// Show content diff between local and remote
			if a.LocalFile == nil || a.State == nil {
				continue
			}

			localContent, err := os.ReadFile(a.LocalFile.AbsPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Failed to read local: %v\n", err)
				continue
			}

			remoteContent, err := convert.ExportDocAsMarkdown(client, a.State.DriveFileID, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Failed to export remote: %v\n", err)
				continue
			}

			diffs := dmp.DiffMain(string(remoteContent), string(localContent), true)
			if len(diffs) == 1 && diffs[0].Type == diffmatchpatch.DiffEqual {
				continue // actually identical after export
			}

			fmt.Printf("--- remote: %s\n+++ local:  %s\n", a.RelPath, a.RelPath)
			fmt.Println(dmp.DiffPrettyText(diffs))
			shown++

		case sync.ActionCreateRemote:
			fmt.Printf("+++ new local: %s\n", a.RelPath)
			shown++

		case sync.ActionCreateLocal:
			fmt.Printf("+++ new remote: %s\n", a.RelPath)
			shown++

		case sync.ActionDeleteLocal, sync.ActionDeleteRemote:
			fmt.Printf("--- deleted: %s\n", a.RelPath)
			shown++
		}
	}

	if shown == 0 && filterPath == "" {
		fmt.Println("No differences found.")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
