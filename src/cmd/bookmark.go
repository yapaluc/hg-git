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
	var rev string
	var cmd = &cobra.Command{
		Use:     "bookmark <name> [-d delete] [-r rev]",
		Short:   "Bookmark (branch) management.",
		Aliases: []string{"book"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runBookmark(args, delete, rev)
		},
	}
	cmd.Flags().BoolVarP(&delete, "delete", "d", false, "Delete the bookmark")
	cmd.Flags().StringVarP(&rev, "rev", "r", "", "Revision to bookmark (not relevant for delete)")
	return cmd
}

func runBookmark(args []string, delete bool, rev string) error {
	if delete {
		return deleteBranches(args)
	}
	branchName := args[0]
	return createBookmark(branchName, rev)
}

func createBookmark(branchName string, rev string) error {
	var cmd string
	if rev != "" {
		currBranch, err := git.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("getting current branch: %w", err)
		}
		if branchName == currBranch {
			err := updateRev(currBranch, &currBranch)
			if err != nil {
				return fmt.Errorf("switching before creating branch %q: %w", branchName, err)
			}
		}

		cmd = fmt.Sprintf(
			"git branch %s %s -f",
			shellescape.Quote(branchName),
			shellescape.Quote(rev),
		)
	} else {
		cmd = fmt.Sprintf("git switch -C %s", shellescape.Quote(branchName))
	}

	_, err := shell.Run(shell.Opt{StreamOutputToStdout: true}, cmd)
	if err != nil {
		return fmt.Errorf("creating branch: %w", err)
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
