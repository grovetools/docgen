package cmd

import (
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize documentation between notebook and repository",
		Long: `Sync commands allow you to copy documentation files between your notebook's
docgen/docs directory and the repository's docs directory.

Use 'sync to-repo' to publish finalized docs from notebook to repository.
Use 'sync from-repo' to import existing docs from repository to notebook.`,
	}

	cmd.AddCommand(newSyncToRepoCmd())
	cmd.AddCommand(newSyncFromRepoCmd())

	return cmd
}
