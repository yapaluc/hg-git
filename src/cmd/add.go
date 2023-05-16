package cmd

import (
	"fmt"
	"strings"

	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <files>",
		Short: "Alias of git add.",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runAdd,
	}
}

func runAdd(cmd *cobra.Command, args []string) error {
	quotedArgs := lo.Map(args, func(arg string, _ int) string {
		return shellescape.Quote(arg)
	})
	cmdStr := fmt.Sprintf("git add %s", strings.Join(quotedArgs, " "))
	_, err := shell.Run(shell.Opt{StreamOutputToStdout: true}, cmdStr)
	return err
}
