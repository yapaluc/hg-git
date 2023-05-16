package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "update <rev>",
		Short:   "Checkout the given rev. Rev can be a branch name or a commit hash. Snaps to a branch name if possible.",
		Aliases: []string{"up"},
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runUpdate(args)
		},
	}
	return cmd
}

func runUpdate(args []string) error {
	rev := args[0]
	return updateRev(rev, nil)
}

func updateRev(rev string, excludeBranch *string) error {
	if rev == "" {
		return fmt.Errorf("rev not provided")
	}

	branchNameResolution, err := git.ResolveBranchName(rev, excludeBranch)
	if err != nil {
		return fmt.Errorf("resolving rev %q to branch name: %w", rev, err)
	}

	if branchNameResolution.BranchName != "" {
		_, err = shell.Run(shell.Opt{StreamOutputToStdout: true}, fmt.Sprintf(
			"git switch %s",
			shellescape.Quote(branchNameResolution.BranchName),
		))
		return err
	}

	_, err = shell.Run(shell.Opt{StreamOutputToStdout: true}, fmt.Sprintf(
		"git switch --detach %s",
		shellescape.Quote(branchNameResolution.CommitHash),
	))
	return err
}
