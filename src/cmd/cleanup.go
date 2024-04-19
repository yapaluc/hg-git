package cmd

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Cleanup merged branches and rebase their descendants on master.",
		Args:  cobra.NoArgs,
		RunE:  runCleanup,
	}
}

func runCleanup(cmd *cobra.Command, args []string) error {
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}

	prunedBranches, err := pruneBranches()
	if err != nil {
		return err
	}

	repoData, err := git.NewRepoData()
	if err != nil {
		return fmt.Errorf("getting repo data: %w", err)
	}

	masterNode := repoData.BranchNameToNode[repoData.MasterBranch]
	branchToMergeArgs := make(map[string][]mergeArg)
	for _, prunedBranch := range prunedBranches {
		prunedNode, ok := repoData.BranchNameToNode[prunedBranch]
		if !ok {
			continue
		}
		branchToMergeArgs[prunedBranch], err = getMergeArgsForCleanup(prunedNode, masterNode)
		if err != nil {
			return fmt.Errorf(
				"getting merge args for cleanup of pruned branch %q: %w",
				prunedBranch,
				err,
			)
		}
	}

	err = pull()
	if err != nil {
		return fmt.Errorf("pulling latest changes: %w", err)
	}

	for prunedBranch, mergeArgs := range branchToMergeArgs {
		color.Green("Pruning branch: %s", prunedBranch)

		// Delete the local branch.
		err := deleteBranches([]string{prunedBranch})
		if err != nil {
			return fmt.Errorf("deleting local pruned branch %q: %w", prunedBranch, err)
		}

		// Restack.
		err = executeRestack(mergeArgs)
		if err != nil {
			return fmt.Errorf("executing restack of pruned branch %q: %w", prunedBranch, err)
		}
	}

	// Checkout the original branch or master if the original branch was pruned.
	branchToCheckout := currentBranch
	if lo.Contains(prunedBranches, currentBranch) {
		branchToCheckout = repoData.MasterBranch
	}
	return updateRev(branchToCheckout, nil)
}

const prunePrefix = " * [pruned] origin/"

// Prune local tracking branches that don't exist on the remote.
func pruneBranches() ([]string, error) {
	lines, err := shell.RunAndCollectLines(
		shell.Opt{StreamOutputToStdout: true, PrintCommand: true},
		"git remote prune origin",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"pruning local tracking branches that don't exist on the remote: %w",
			err,
		)
	}

	var prunedBranches []string
	for _, line := range lines {
		if strings.HasPrefix(line, prunePrefix) {
			prunedBranches = append(prunedBranches, strings.TrimPrefix(line, prunePrefix))
		}
	}
	return prunedBranches, nil
}

// Run a depth-first search from the pruned branch to find
// the children branches and the order in which to merge them.
func getMergeArgsForCleanup(
	prunedNode *git.TreeNode,
	masterNode *git.TreeNode,
) ([]mergeArg, error) {
	var mergeArgs []mergeArg
	var dfs func(node *git.TreeNode, parent *git.TreeNode) error
	dfs = func(node *git.TreeNode, parent *git.TreeNode) error {
		// Switch out the pruned node for master.
		if parent == prunedNode {
			parent = masterNode
		}

		if parent != nil {
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
		}
		for _, child := range node.BranchChildren {
			err := dfs(child, node)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err := dfs(prunedNode, nil /* parent */)
	if err != nil {
		return nil, err
	}

	return mergeArgs, nil
}
