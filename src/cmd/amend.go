package cmd

import (
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newAmendCmd() *cobra.Command {
	var message string
	var force bool
	var empty bool
	var cmd = &cobra.Command{
		Use:   "amend [-m message | -f force] [-e empty]",
		Short: "Commits changes as a new commit on the current branch and restacks descendant branches via merges (not rebases).",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _args []string) error {
			return runAmend(message, force, empty)
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to commit with")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Use a default message")
	cmd.Flags().
		BoolVarP(&empty, "empty", "e", false, "Create an empty commit. The use case is to force a push to remote to trigger a build.")
	cmd.MarkFlagsMutuallyExclusive("message", "force")
	return cmd
}

func runAmend(message string, force bool, empty bool) error {
	msg := message
	if msg == "" && force {
		msg = "update"
	} else if empty {
		msg = "empty commit"
	}
	if msg == "" {
		return fmt.Errorf("-m is required or specify -f to use a default")
	}

	branch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}

	repoData, err := git.NewRepoData(git.RepoDataIncludeCommitMetadata)
	if err != nil {
		return err
	}

	// Prepare merge arguments.
	currNode := repoData.BranchNameToNode[branch]
	mergeArgs, err := getMergeArgs(currNode)
	if err != nil {
		return fmt.Errorf("finding restack order: %w", err)
	}

	// Commit.
	var allowEmptyFlag string
	if empty {
		allowEmptyFlag = " --allow-empty"
	}
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true, PrintCommand: true},
		fmt.Sprintf("git add --all && git commit -m %s%s", shellescape.Quote(msg), allowEmptyFlag),
	)
	if err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	// Restack.
	err = executeRestack(mergeArgs)
	if err != nil {
		return fmt.Errorf("executing restack: %w", err)
	}

	// Checkout original branch.
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true, PrintCommand: true},
		fmt.Sprintf("git switch %s", shellescape.Quote(branch)),
	)
	if err != nil {
		return fmt.Errorf("checking out original branch %q: %w", branch, err)
	}

	return nil
}

type mergeArg struct {
	branchToMerge        string
	branchToReceiveMerge string
}

// Run a depth-first search from the current branch to find
// the children branches and the order in which to merge them.
func getMergeArgs(currNode *git.TreeNode) ([]mergeArg, error) {
	var mergeArgs []mergeArg
	var dfs func(node *git.TreeNode, parent *git.TreeNode) error
	dfs = func(node *git.TreeNode, parent *git.TreeNode) error {
		if parent != nil {
			if len(parent.CommitMetadata.CleanedBranchNames()) == 0 {
				return fmt.Errorf(
					"expected branch names on commit %s",
					parent.CommitMetadata.CommitHash,
				)
			}
			if len(node.CommitMetadata.CleanedBranchNames()) == 0 {
				return fmt.Errorf(
					"expected branch names on commit %s",
					node.CommitMetadata.CommitHash,
				)
			}
			mergeArgs = append(mergeArgs, mergeArg{
				branchToMerge:        parent.CommitMetadata.CleanedBranchNames()[0],
				branchToReceiveMerge: node.CommitMetadata.CleanedBranchNames()[0],
			})
		}
		for _, child := range node.BranchChildren {
			err := dfs(child, node)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err := dfs(currNode, nil /* parent */)
	if err != nil {
		return nil, err
	}
	return mergeArgs, nil
}

func executeRestack(mergeArgs []mergeArg) error {
	for _, mergeArg := range mergeArgs {
		_, err := shell.Run(
			shell.Opt{StreamOutputToStdout: true, PrintCommand: true},
			fmt.Sprintf(
				"git switch %s && git merge %s -m %s",
				shellescape.Quote(mergeArg.branchToReceiveMerge),
				shellescape.Quote(mergeArg.branchToMerge),
				shellescape.Quote(
					fmt.Sprintf("Sync changes from upstream (%s)", mergeArg.branchToMerge),
				),
			),
		)
		if err == nil {
			continue
		}
		err = maybePromptForMergeConflictResolution("git commit --no-edit")
		if err != nil {
			return fmt.Errorf("waiting for merge conflict resolution: %w", err)
		}
	}
	return nil
}

func maybePromptForMergeConflictResolution(resolutionCommand string) error {
	for {
		lines, err := shell.RunAndCollectLines(
			shell.Opt{},
			"git diff --name-only --diff-filter=U",
		)
		if err != nil {
			return fmt.Errorf("checking for merge conflicts: %w", err)
		}
		if len(lines) == 0 {
			break
		}
		color.Red("Merge conflicts hit:")
		for _, line := range lines {
			color.Red("  " + line)
		}
		color.Red("Resolve the merge conflicts and press any character to continue.")
		color.Red("You don't need to add files or continue the merge/rebase.")
		err = waitForUserInput()
		if err != nil {
			return fmt.Errorf("waiting for user input: %w", err)
		}
		color.Green("Continuing")
		_, err = shell.Run(
			shell.Opt{StreamOutputToStdout: true, PrintCommand: true},
			"git add --all && "+resolutionCommand,
		)
		if err != nil {
			return fmt.Errorf("continuing merge/rebase: %w", err)
		}
	}
	return nil
}

func waitForUserInput() (err error) {
	// save terminal state to restore later
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState) // restore terminal state on exit

	// read one byte from stdin
	var b = make([]byte, 1)
	_, err = os.Stdin.Read(b)
	if err != nil {
		return fmt.Errorf("failed to read byte: %w", err)
	}
	return nil
}
