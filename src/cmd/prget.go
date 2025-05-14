package cmd

import (
	"fmt"

	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
	"github.com/yapaluc/hg-git/src/shell"
)

func newPrgetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prget <pr url | pr num>",
		Short: "Check out a PR from GitHub.",
		Args:  cobra.ExactArgs(1),
		RunE:  runPrget,
	}
}

func runPrget(_ *cobra.Command, args []string) error {
	// TODO - restack after running this
	prURLOrNum := args[0]
	return checkoutPR(prURLOrNum)
}

func checkoutPR(prURLOrNumOrBranch string) error {
	_, err := shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf(
			"gh pr checkout %s",
			shellescape.Quote(prURLOrNumOrBranch),
		),
	)
	if err != nil {
		return fmt.Errorf("running gh pr checkout: %w", err)
	}
	return nil
}
