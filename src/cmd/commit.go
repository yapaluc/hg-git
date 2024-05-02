package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
)

func newCommitCmd() *cobra.Command {
	var msg string
	var cmd = &cobra.Command{
		Use:   "commit [-m message]",
		Short: "Stage all files and commit.",
		Args:  cobra.NoArgs,
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

func runCommit(_ []string, msg string) error {
	branch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	masterBranch, err := git.GetMasterBranch()
	if err != nil {
		return fmt.Errorf("getting master branch: %w", err)
	}

	err = commitAll(msg)
	if err != nil {
		return err
	}

	if branch == masterBranch {
		return nil
	}

	// Populate branch description if this is a new branch
	currDesc, err := getBranchDescriptionWithFallback(branch)
	if err != nil {
		return fmt.Errorf("could not get current branch description: %w", err)
	}

	err = writeBranchDescription(branch, currDesc)
	if err != nil {
		return fmt.Errorf("writing branch description: %w", err)
	}
	return nil
}

func commitAll(msg string) error {
	cmdStr := fmt.Sprintf("git add --all && git commit -m %s", shellescape.Quote(msg))
	_, err := shell.Run(shell.Opt{StreamOutputToStdout: true}, cmdStr)
	if err != nil {
		return fmt.Errorf("running commit: %w", err)
	}
	return nil
}
