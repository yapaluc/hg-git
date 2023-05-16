package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
)

func newCommitCmd() *cobra.Command {
	var msg string
	var cmd = &cobra.Command{
		Use:   "commit [-m message]",
		Short: "Stage all files and commit.",
		RunE: func(_ *cobra.Command, args []string) error {
			return runCommit(args, msg)
		},
	}
	cmd.Flags().StringVarP(&msg, "message", "m", "", "Commit message")
	if err := cmd.MarkFlagRequired("message"); err != nil {
		panic("failed to mark --message flag as required")
	}
	return cmd
}

func runCommit(args []string, msg string) error {
	cmdStr := fmt.Sprintf("git add --all && git commit -m %s", shellescape.Quote(msg))
	_, err := shell.Run(shell.Opt{StreamOutputToStdout: true}, cmdStr)
	return err
}
