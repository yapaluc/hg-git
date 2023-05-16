package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/git"

	"github.com/spf13/cobra"
)

func newPrevCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prev",
		Short: "Checks out the parent branch.",
		RunE:  runPrev,
	}
}

func runPrev(_ *cobra.Command, args []string) error {
	repoData, err := git.NewRepoData()
	if err != nil {
		return err
	}

	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}

	node, ok := repoData.BranchNameToNode[currBranch]
	if !ok {
		return fmt.Errorf("missing node for branch %q", currBranch)
	}

	parent := node.BranchParent
	if parent == nil {
		return fmt.Errorf("no parent found for branch %q", currBranch)
	}
	return updateRev(parent.CommitMetadata.CommitHash, nil)
}
