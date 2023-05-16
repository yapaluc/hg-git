package cmd

import (
	"fmt"

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
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runBookmark(args, delete)
		},
	}
	cmd.Flags().BoolVarP(&delete, "delete", "d", false, "Delete the bookmark")
	return cmd
}

func runBookmark(args []string, delete bool) error {
	branchName := args[0]

	if delete {
		return deleteBranch(branchName)
	}

	_, err := shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git switch -c %s", shellescape.Quote(branchName)),
	)
	if err != nil {
		return fmt.Errorf("switching to branch %q: %w", branchName, err)
	}
	return nil
}

func deleteBranch(branchName string) error {
	err := updateRev(branchName, &branchName)
	if err != nil {
		return fmt.Errorf("switching before deleting branch %q: %w", branchName, err)
	}
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git branch -D %s", shellescape.Quote(branchName)),
	)
	if err != nil {
		return fmt.Errorf("deleting branch %q: %w", branchName, err)
	}
	return nil
}
