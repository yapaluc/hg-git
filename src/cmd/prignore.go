package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yapaluc/hg-git/src/shell"
)

func newPrignoreCmd() *cobra.Command {
	var off bool
	cmd := &cobra.Command{
		Use:   "prignore <branch>",
		Short: "Mark a branch to be ignored by the submit command.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runPrignore(args, off)
		},
	}
	cmd.Flags().BoolVarP(&off, "off", "o", false, "Turn ignoring off")
	return cmd
}

func runPrignore(args []string, off bool) error {
	branchName := args[0]
	val := "true"
	if off {
		val = "false"
	}
	_, err := shell.Run(
		shell.Opt{},
		fmt.Sprintf(
			"git config branch.%s.hggit.prignore %s",
			branchName,
			val,
		),
	)
	if err != nil {
		return fmt.Errorf("running git config: %w", err)
	}
	return nil
}

func isPrIgnored(branchName string) bool {
	out, err := shell.Run(
		shell.Opt{},
		fmt.Sprintf(
			"git config branch.%s.hggit.prignore",
			branchName,
		),
	)
	// git config exits with code 1 if the config doesn't exist.
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}
