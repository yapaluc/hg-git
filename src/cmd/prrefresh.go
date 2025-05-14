package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yapaluc/hg-git/src/git"
)

func newPrRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prrefresh",
		Short: "Refresh the current branch with the PR from GitHub.",
		Long:  "Refresh the current branch with the PR from GitHub. Equivalent to `hg prget` followed by `hg prsync`.",
		Args:  cobra.NoArgs,
		RunE:  runPrrefresh,
	}
	return cmd
}

func runPrrefresh(_ *cobra.Command, args []string) error {
	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	err = checkoutPR(currBranch)
	if err != nil {
		return fmt.Errorf("checking out PR for branch %q: %w", currBranch, err)
	}
	err = syncPR(currBranch)
	if err != nil {
		return fmt.Errorf("syncing PR for branch %q: %w", currBranch, err)
	}

	return nil
}
