package cmd

import (
	"fmt"

	"github.com/alessio/shellescape"
	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/spf13/cobra"
)

func newRebaseCmd() *cobra.Command {
	var source string
	var dest string
	var cmd = &cobra.Command{
		Use:   "rebase -s source -d dest",
		Short: "Rebases the given branch and its descendants onto the given branch. Rebase is done with a merge instead of an actual rebase.",
		RunE: func(_ *cobra.Command, args []string) error {
			return runRebase(args, source, dest)
		},
	}
	cmd.Flags().StringVarP(&source, "source", "s", "", "Source rev")
	cmd.Flags().StringVarP(&dest, "dest", "d", "", "Destination rev")
	return cmd
}

func runRebase(args []string, source, dest string) error {
	// TODO - scenarios to support:
	// - rebasing random part of stack onto a random commit (requires reverting branches starting from the root)
	repoData, err := git.NewRepoData()
	if err != nil {
		return err
	}

	// Save current branch for later.
	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}

	sourceBranch, err := resolveRevToBranchName(source)
	if err != nil {
		return fmt.Errorf("could not resolve rev %q to branch name", source)
	}

	destBranch, err := resolveRevToBranchName(dest)
	if err != nil {
		return fmt.Errorf("could not resolve rev %q to branch name", dest)
	}

	sourceNode := repoData.BranchNameToNode[sourceBranch]
	destNode := repoData.BranchNameToNode[destBranch]

	if !sourceNode.BranchParent.CommitMetadata.IsEffectiveMaster() {
		return fmt.Errorf("source is not the root of the stack")
	}

	// Run series of merges
	mergeArgs, err := getMergeArgsForRebase(sourceNode, destNode)
	if err != nil {
		return fmt.Errorf(
			"getting merge args for rebase on master: %w",
			err,
		)
	}

	// Execute merges.
	err = executeRestack(mergeArgs)
	if err != nil {
		return fmt.Errorf("executing merges for rebase on master: %w", err)
	}

	// Checkout original branch.
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true, PrintCommand: true},
		fmt.Sprintf("git switch %s", shellescape.Quote(currBranch)),
	)
	if err != nil {
		return fmt.Errorf("checking out original branch %q: %w", currBranch, err)
	}
	return nil
}

func resolveRevToBranchName(rev string) (string, error) {
	branchNameResolution, err := git.ResolveBranchName(rev, nil)
	if err != nil {
		return "", fmt.Errorf("resolving rev %q to branch name: %w", rev, err)
	}
	if branchNameResolution.BranchName == "" {
		return "", fmt.Errorf("rev %q has no branch name", rev)
	}
	return branchNameResolution.BranchName, nil
}

// Run a depth-first search from the root of stack to find
// the children branches and the order in which to merge them.
func getMergeArgsForRebase(
	stackRootNode *git.TreeNode,
	masterNode *git.TreeNode,
) ([]mergeArg, error) {
	var mergeArgs []mergeArg
	var dfs func(node *git.TreeNode, parent *git.TreeNode) error
	dfs = func(node *git.TreeNode, parent *git.TreeNode) error {
		if len(parent.CommitMetadata.CleanedBranchNames()) == 0 {
			return fmt.Errorf(
				"expected branch names on commit %s",
				parent.CommitMetadata.CommitHash,
			)
		}
		if len(node.CommitMetadata.CleanedBranchNames()) == 0 {
			return fmt.Errorf(
				"expected branch names on commit %s",
				node.CommitMetadata.CommitHash,
			)
		}
		mergeArgs = append(mergeArgs, mergeArg{
			branchToMerge:        parent.CommitMetadata.CleanedBranchNames()[0],
			branchToReceiveMerge: node.CommitMetadata.CleanedBranchNames()[0],
		})
		for _, child := range node.BranchChildren {
			err := dfs(child, node)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err := dfs(stackRootNode, masterNode)
	if err != nil {
		return nil, err
	}

	return mergeArgs, nil
}
