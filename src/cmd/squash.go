package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/spf13/cobra"
)

func newSquashCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "squash [branch name]",
		Short:   "Squash the current branch into one commit.",
		Long:    "Squash the current branch into one commit. Works across merges.",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"sq"},
		RunE: func(_ *cobra.Command, args []string) error {
			return runSquash(args)
		},
	}
}

func runSquash(args []string) error {
	repoData, err := git.NewRepoData()
	if err != nil {
		return err
	}

	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}

	var branchName string
	if len(args) > 0 {
		branchName = args[0]
	} else {
		branchName = currBranch
	}

	node, ok := repoData.BranchNameToNode[branchName]
	if !ok {
		return fmt.Errorf("missing node for branch %q", branchName)
	}

	parentBranch := node.BranchParent
	if parentBranch == nil {
		return fmt.Errorf("missing parent branch for branch %q", branchName)
	}

	// Get patch.
	parentBranchName := parentBranch.CommitMetadata.CleanedBranchNames()[0]
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git diff %s %s > diff.patch", parentBranchName, branchName),
	)
	if err != nil {
		return fmt.Errorf("getting patch: %w", err)
	}

	// Checkout parent branch.
	err = updateRev(parentBranchName, nil)
	if err != nil {
		return fmt.Errorf("checking out parent branch %q: %w", parentBranchName, err)
	}

	// Recreate branch name at the parent.
	err = createBookmark(branchName)
	if err != nil {
		return fmt.Errorf("creating bookmark for branch %q at the parent: %w", branchName, err)
	}

	// Apply the patch.
	_, err = shell.Run(shell.Opt{StreamOutputToStdout: true}, "git apply diff.patch")
	if err != nil {
		return fmt.Errorf("applying patch: %w", err)
	}

	// Remove the patch file.
	_, err = shell.Run(shell.Opt{}, "rm diff.patch")
	if err != nil {
		return fmt.Errorf("removing patch: %w", err)
	}

	// Commit using the existing title.
	branchTitle := node.CommitMetadata.BranchDescription.Title
	err = commitAll(branchTitle)
	if err != nil {
		return err
	}

	// Checkout the original branch.
	if branchName != currBranch {
		err = updateRev(currBranch, nil)
		if err != nil {
			return fmt.Errorf("checking out original branch %q: %w", currBranch, err)
		}
	}

	return nil
}
