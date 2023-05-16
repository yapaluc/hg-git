package cmd

import (
	"fmt"
	"strings"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

func newRevertCmd() *cobra.Command {
	var rev string
	var cmd = &cobra.Command{
		Use:   "revert [-r rev] <filepath>...",
		Short: "Revert file(s) to a given revision.",
		Long:  "Revert file(s) to a given revision. When specifying .^ as the revision, file(s) will be reverted to their state in the parent branch (not parent commit).",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runRevert(args, rev)
		},
	}
	cmd.Flags().StringVarP(&rev, "rev", "r", "", "Revision to diff against")
	return cmd
}

func runRevert(args []string, rev string) error {
	filepaths := args

	// Revert to a given revision.
	if rev != "" {
		if rev == ".^" {
			// Find the previous branch.
			repoData, err := git.NewRepoData()
			if err != nil {
				return err
			}

			currBranch, err := git.GetCurrentBranch()
			if err != nil {
				return err
			}
			rev = repoData.BranchNameToNode[currBranch].BranchParent.CommitMetadata.CommitHash
		}
		_, err := shell.Run(
			shell.Opt{StreamOutputToStdout: true},
			fmt.Sprintf(
				"git restore -s %s %s",
				rev,
				strings.Join(
					lo.Map(filepaths, func(f string, _ int) string { return shellescape.Quote(f) }),
					" ",
				),
			),
		)
		if err != nil {
			return fmt.Errorf("reverting files: %w", err)
		}
		return nil
	}

	// Revert unstaged changes.
	_, err := shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf(
			"git restore --staged --worktree %s",
			strings.Join(
				lo.Map(filepaths, func(f string, _ int) string { return shellescape.Quote(f) }),
				" ",
			),
		),
	)
	if err != nil {
		return fmt.Errorf("reverting files: %w", err)
	}
	return nil
}
