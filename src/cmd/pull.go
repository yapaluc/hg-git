package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Pull master from remote.",
		Args:  cobra.NoArgs,
		RunE:  runPull,
	}
}

func runPull(_ *cobra.Command, args []string) error {
	return pull()
}

func pull() error {
	masterBranch, err := git.GetMasterBranch()
	if err != nil {
		return fmt.Errorf("getting master branch: %w", err)
	}

	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf(
			"git -c color.ui=always fetch origin %s:%s --update-head-ok",
			masterBranch,
			masterBranch,
		),
	)
	if err != nil {
		return fmt.Errorf("running git fetch: %w", err)
	}
	return nil
}
