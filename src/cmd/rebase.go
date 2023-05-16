package cmd

import (
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/spf13/cobra"
)

func newRebaseCmd() *cobra.Command {
	var source string
	var dest string
	var cmd = &cobra.Command{
		Use:   "rebase -s source -s dest",
		Short: "Rebases the given branch and its descendants onto the given branch. Rebase is done with a merge instead of an actual rebase.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runRebase(args, source, dest)
		},
	}
	cmd.Flags().StringVarP(&source, "source", "s", "", "Source rev")
	cmd.Flags().StringVarP(&source, "dest", "d", "", "Destination rev")
	return cmd
}

func runRebase(args []string, source, dest string) error {
	// TODO - scenarios to support:
	// - rebasing root of stack onto master (requires a merge on master and restack)
	// - rebasing random part of stack onto a random commit (requires reverting branches starting from the root)
	_, err := shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		"git -c color.ui=always fetch origin main:main",
	)
	return err
}
