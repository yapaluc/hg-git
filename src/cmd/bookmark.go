package cmd

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
)

func newBookmarkCmd() *cobra.Command {
	var delete bool
	var cmd = &cobra.Command{
		Use:     "bookmark <name> [-d delete]",
		Short:   "Bookmark (branch) management.",
		Aliases: []string{"book"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runBookmark(args, delete)
		},
	}
	cmd.Flags().BoolVarP(&delete, "delete", "d", false, "Delete the bookmark")
	return cmd
}

func runBookmark(args []string, delete bool) error {
	if delete {
		return deleteBranches(args)
	}

	branchName := args[0]
	_, err := shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git switch -c %s", shellescape.Quote(branchName)),
	)
	if err != nil {
		return fmt.Errorf("switching to branch %q: %w", branchName, err)
	}
	return nil
}

func deleteBranches(branchNames []string) error {
	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	if lo.Contains(branchNames, currBranch) {
		err := updateRev(currBranch, &currBranch)
		if err != nil {
			return fmt.Errorf("switching before deleting branch %q: %w", currBranch, err)
		}
	}
	quotedBranchNames := lo.Map(
		branchNames,
		func(branchName string, _ int) string {
			return shellescape.Quote(branchName)
		},
	)
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git branch -D %s", strings.Join(quotedBranchNames, " ")),
	)
	if err != nil {
		return fmt.Errorf("deleting branches %v: %w", branchNames, err)
	}
	return nil
}
