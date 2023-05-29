package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/git"

	"github.com/spf13/cobra"
)

func newNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Checks out the child branch.",
		Args:  cobra.NoArgs,
		RunE:  runNext,
	}
}

func runNext(_ *cobra.Command, args []string) error {
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

	children := node.BranchChildren
	if len(children) == 0 {
		return fmt.Errorf("no child found for branch %q", currBranch)
	}
	if len(children) > 1 {
		return fmt.Errorf("multiple children found for branch %q", currBranch)
	}
	for _, child := range children {
		return updateRev(child.CommitMetadata.CommitHash, nil)
	}
	return fmt.Errorf("invariant violation")
}
