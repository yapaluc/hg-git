package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/git"
	"golang.org/x/exp/maps"

	"github.com/spf13/cobra"
)

func newTopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "top",
		Short: "Checks out the top branch of the current stack.",
		Args:  cobra.NoArgs,
		RunE:  runTop,
	}
}

func runTop(_ *cobra.Command, args []string) error {
	repoData, err := git.NewRepoData(
		git.RepoDataIncludeCommitMetadata,
	)
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

	for len(node.BranchChildren) > 0 {
		if len(node.BranchChildren) > 1 {
			return fmt.Errorf(
				"ambiguous command: branch %q has more than one child",
				node.CommitMetadata.CleanedBranchNames()[0],
			)
		}
		node = maps.Values(node.BranchChildren)[0]
	}

	return updateRev(node.CommitMetadata.CommitHash, nil)
}
