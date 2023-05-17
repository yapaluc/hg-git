package cmd

import (
	"fmt"
	"strings"

	"github.com/alessio/shellescape"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"
)

func newDiffCmd() *cobra.Command {
	var rev string
	var cmd = &cobra.Command{
		Use:   "diff [-r rev] [<filepath>...]",
		Short: "Alias of git diff.",
		Long: `Alias of git diff. Supported commands:
			hg diff
			hg diff file.txt
			hg diff -r .^ file.txt
		`,
		RunE: func(_ *cobra.Command, args []string) error {
			return runDiff(args, rev)
		},
	}
	cmd.Flags().StringVarP(&rev, "rev", "r", "", "Revision to diff against")
	return cmd
}

func runDiff(args []string, rev string) error {
	filesStr := strings.Join(
		lo.Map(args, func(f string, _ int) string { return shellescape.Quote(f) }),
		" ",
	)

	// No rev.
	if rev == "" {
		_, err := shell.Run(
			shell.Opt{StreamOutputToStdout: true},
			fmt.Sprintf("git -c color.ui=always diff %s", filesStr),
		)
		if err != nil {
			return fmt.Errorf("running git diff: %w", err)
		}
		return nil
	}

	resolvedRev, err := git.ResolveRev(rev)
	if err != nil {
		return fmt.Errorf("resolving rev %q: %w", rev, err)
	}

	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git -c color.ui=always diff %s..HEAD %s", resolvedRev, filesStr),
	)
	if err != nil {
		return fmt.Errorf("running git diff: %w", err)
	}
	return nil
}
