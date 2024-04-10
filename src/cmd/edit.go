package cmd

import (
	"fmt"
	"strings"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "edit [branch name]",
		Short: "Edits the branch description.",
		Long: `Edits the branch description of the given branch (defaults to the current branch).
			First line should be the desired PR title, followed by an empty line, followed by the desired PR body.
			Optionally specify a rev (branch name or commit hash).`,
		Aliases: []string{"e"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runEdit(args)
		},
	}
	return cmd
}

func runEdit(args []string) error {
	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}
	var rev string
	if len(args) == 0 {
		rev = currBranch
	} else {
		rev = args[0]
	}
	branchNameResolution, err := git.ResolveBranchName(rev, nil)
	if err != nil {
		return fmt.Errorf("resolving rev %q to branch name: %w", rev, err)
	}
	if branchNameResolution.BranchName == "" {
		return fmt.Errorf("rev %q has no branch name", rev)
	}
	err = editByBranchName(branchNameResolution.BranchName)
	if err != nil {
		return fmt.Errorf(
			"editing description of branch %q: %w",
			branchNameResolution.BranchName,
			err,
		)
	}
	return nil
}

const template = `%s
%s Please edit the description for the branch
%s   %s
%s Lines starting with '%s' will be stripped.
`

const editFilePrefix = "EDIT_BRANCH_DESC_"

func editByBranchName(branchName string) error {
	currDesc, err := getBranchDescriptionWithFallback(branchName)
	if err != nil {
		return fmt.Errorf("could not get current branch description: %w", err)
	}

	commentChar, err := shell.Run(
		shell.Opt{StripTrailingNewline: true},
		"git config core.commentchar",
	)
	if err != nil {
		// git config returns a non-zero exit code if the config doesn't exist
		commentChar = "#"
	}

	currDesc = strings.TrimSpace(currDesc)
	newDesc, err := shell.OpenEditor(
		fmt.Sprintf(
			template,
			currDesc,
			commentChar,
			commentChar,
			branchName,
			commentChar,
			commentChar,
		),
		editFilePrefix,
		commentChar,
	)
	if err != nil {
		return fmt.Errorf("opening file for editing: %w", err)
	}
	if newDesc == "" {
		return fmt.Errorf("user did not edit description")
	}

	err = writeBranchDescription(branchName, newDesc)
	if err != nil {
		return fmt.Errorf("writing branch description: %w", err)
	}
	return nil
}

func writeBranchDescription(branchName string, branchDesc string) error {
	_, err := shell.Run(
		shell.Opt{},
		fmt.Sprintf(
			"git config branch.%s.description %s",
			branchName,
			shellescape.Quote(branchDesc),
		),
	)
	return err
}

func getBranchDescriptionWithFallback(branchName string) (string, error) {
	currDesc, err := shell.Run(
		shell.Opt{},
		fmt.Sprintf("git config branch.%s.description", shellescape.Quote(branchName)),
	)
	if err == nil {
		return currDesc, nil
	}
	// git config exits with code 1 if there are no branch descriptions.
	// Fallback to pre-populating the description with the current commit message.
	prettyFormat := "%s"
	commitTitle, err := shell.Run(
		shell.Opt{},
		fmt.Sprintf("git log --format=%s -n 1 %s", prettyFormat, shellescape.Quote(branchName)),
	)
	if err != nil {
		return "", fmt.Errorf("could not read commit title")
	}
	return commitTitle, nil
}
