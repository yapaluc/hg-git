package cmd

import (
	"fmt"

	"github.com/alessio/shellescape"
	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/spf13/cobra"
)

func newSquashCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "squash",
		Short:   "Squash the current branch into one commit.",
		Args:    cobra.NoArgs,
		Aliases: []string{"sq"},
		RunE: func(_ *cobra.Command, args []string) error {
			return runSquash(args)
		},
	}
	return cmd
}

func runSquash(args []string) error {
	repoData, err := git.NewRepoData()
	if err != nil {
		return err
	}

	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}

	node, ok := repoData.BranchNameToNode[currBranch]
	if !ok {
		return fmt.Errorf("missing node for branch %q", currBranch)
	}

	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf(
			`GIT_EDITOR='f() { if [ "$(basename $1)" = "git-rebase-todo" ]; then sed -i "" "2,\$s/pick/squash/" $1; else true; fi }; f' git rebase -i %s %s`,
			shellescape.Quote(node.BranchParent.CommitMetadata.CleanedBranchNames()[0]),
			shellescape.Quote(node.CommitMetadata.CleanedBranchNames()[0]),
		),
	)
	if err != nil {
		return fmt.Errorf("could not run squash")
	}
	return err
}
