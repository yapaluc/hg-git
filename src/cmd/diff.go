package cmd

import (
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var rev string
	var cmd = &cobra.Command{
		Use:   "diff [-r rev] [<filepath>...]",
		Short: "Alias of git diff.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runDiff(args, rev)
		},
	}
	cmd.Flags().StringVarP(&rev, "rev", "r", "", "Revision to diff against")
	return cmd
}

func runDiff(args []string, rev string) error {
	// TODO
	return nil
}
