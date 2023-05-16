package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func Execute() {
	rootCmd := &cobra.Command{
		Use:   "hg",
		Short: "hg is set of commands for emulating a subset of Mercurial commands on a Git repository, as well as interacting with GitHub Pull Requests.",
	}
	rootCmd.AddCommand(
		newAddCmd(),
		newAmendCmd(),
		newBookmarkCmd(),
		newCleanupCmd(),
		newCommitCmd(),
		newDiffCmd(),
		newEditCmd(),
		newNextCmd(),
		newPrevCmd(),
		newPrsyncCmd(),
		newPullCmd(),
		newRebaseCmd(),
		newRevertCmd(),
		newSmartlogCmd(),
		newStatusCmd(),
		newSubmitCmd(),
		newUpdateCmd(),
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, color.RedString("error: %s", err))
		os.Exit(1)
	}
}