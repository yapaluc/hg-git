package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var change string
	var cmd = &cobra.Command{
		Use:     "status [--change]",
		Short:   "Alias of git status.",
		Aliases: []string{"st"},
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return runStatus(args, change)
		},
	}
	cmd.Flags().
		StringVar(&change, "change", "", "Revision to list changed files (shows changes since the last branch)")
	return cmd
}

func runStatus(args []string, change string) error {
	if change != "" {
		return showChangedFiles(args, change)
	}
	_, err := shell.Run(shell.Opt{StreamOutputToStdout: true}, "git -c color.ui=always status")
	return err
}

func showChangedFiles(_ []string, change string) error {
	repoData, err := git.NewRepoData(
		git.RepoDataIncludeCommitMetadata,
	)
	if err != nil {
		return err
	}

	branchName := change
	if branchName == "." {
		branchName, err = git.GetCurrentBranch()
		if err != nil {
			return err
		}
	}

	node, ok := repoData.BranchNameToNode[branchName]
	if !ok {
		return fmt.Errorf("missing node for branch %q", branchName)
	}

	var parentBranchRef string
	if node.CommitMetadata.IsMaster {
		parentBranchRef = repoData.MasterBranch + "^"
	} else {
		parentBranchRef = node.BranchParent.CommitMetadata.CommitHash
	}

	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf(
			"git diff --name-status %s %s",
			parentBranchRef,
			shellescape.Quote(branchName),
		),
	)
	return err
}
