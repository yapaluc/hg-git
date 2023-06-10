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
		Short: "Rebases the given branch and its descendants onto the given branch. If possible, rebase is done with a merge instead of an actual rebase. For example, when rebasing the root of a stack, a merge is used. When rebasing the middle of a stack, a rebase is used.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return runRebase(args, source, dest)
		},
	}
	cmd.Flags().StringVarP(&source, "source", "s", "", "Source rev")
	cmd.Flags().StringVarP(&dest, "dest", "d", "", "Destination rev")
	return cmd
}

func runRebase(args []string, source, dest string) error {
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

	if sourceNode.BranchParent.CommitMetadata.IsEffectiveMaster() {
		err = rebaseRootOfStack(sourceNode, destNode)
		if err != nil {
			return fmt.Errorf("rebasing root of stack: %w", err)
		}
	} else {
		err = rebaseMiddleOfStack(sourceNode, destNode)
		if err != nil {
			return fmt.Errorf("rebasing middle of stack: %w", err)
		}
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

func rebaseMiddleOfStack(sourceNode *git.TreeNode, destNode *git.TreeNode) error {
	// Get rebase args.
	rebaseArgs, err := getRebaseArgsForRebase(sourceNode, destNode)
	if err != nil {
		return fmt.Errorf("getting rebase args for rebase: %w", err)
	}

	// Run rebases.
	for _, rebaseArg := range rebaseArgs {
		_, err = shell.Run(
			shell.Opt{StreamOutputToStdout: true},
			// -X theirs is for preferring current branch changes during conflicts
			fmt.Sprintf(
				"git checkout %s && git rebase --onto %s %s %s --update-refs --no-edit -X theirs",
				shellescape.Quote(rebaseArg.branchToRebase),
				shellescape.Quote(rebaseArg.targetLocationBranch),
				shellescape.Quote(rebaseArg.oldParentCommitHash),
				shellescape.Quote(rebaseArg.branchToRebase),
			),
		)
		if err == nil {
			continue
		}
		err = maybePromptForMergeConflictResolution("git rebase --continue")
		if err != nil {
			return fmt.Errorf("waiting for merge conflict resolution: %w", err)
		}
	}
	return nil
}

func rebaseRootOfStack(sourceNode *git.TreeNode, destNode *git.TreeNode) error {
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

type rebaseArg struct {
	branchToRebase       string
	targetLocationBranch string
	oldParentCommitHash  string
}

// Run a depth-first search from the source node to find
// the children branches and the order in which to rebase them.
// This is similar to getMergeArgsForRebase except the args include hashes.
func getRebaseArgsForRebase(
	sourceNode *git.TreeNode,
	destNode *git.TreeNode,
) ([]rebaseArg, error) {
	var rebaseArgs []rebaseArg
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
		rebaseArgs = append(rebaseArgs, rebaseArg{
			branchToRebase:       node.CommitMetadata.CleanedBranchNames()[0],
			targetLocationBranch: parent.CommitMetadata.CleanedBranchNames()[0],
			oldParentCommitHash:  node.BranchParent.CommitMetadata.CommitHash,
		})
		for _, child := range node.BranchChildren {
			err := dfs(child, node)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err := dfs(sourceNode, destNode)
	if err != nil {
		return nil, err
	}

	return rebaseArgs, nil
}
