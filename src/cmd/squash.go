package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/spf13/cobra"
)

func newSquashCmd() *cobra.Command {
	var force bool
	var cmd = &cobra.Command{
		Use:     "squash [branch name] [-f force]",
		Short:   "Squash the current branch into one commit.",
		Long:    "Squash the current branch into one commit. Works across merges.",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"sq"},
		RunE: func(_ *cobra.Command, args []string) error {
			return runSquash(args, force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Use a default message")
	return cmd
}

// TODO: update squash command to be stack-aware and work in the middle of a stack

func runSquash(args []string, force bool) error {
	repoData, err := git.NewRepoData()
	if err != nil {
		return err
	}

	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}

	var branchName string
	if len(args) > 0 {
		branchName = args[0]
	} else {
		branchName = currBranch
	}

	commitToStartPatchAt, commitMessage, err := getCommitToStartPatchAtAndCommitMessage(
		repoData,
		branchName,
		force,
	)
	if err != nil {
		return fmt.Errorf("getting squash details for branch %q: %w", branchName, err)
	}

	// Prepare patch file.
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, filePrefix)
	if err != nil {
		return fmt.Errorf("creating temp file for patch: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Get patch.
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git diff-index %s --binary > %s", commitToStartPatchAt, tmpFile.Name()),
	)
	if err != nil {
		return fmt.Errorf("getting patch: %w", err)
	}

	// Checkout parent branch.
	err = updateRev(commitToStartPatchAt, nil)
	if err != nil {
		return fmt.Errorf("checking out parent branch %q: %w", commitToStartPatchAt, err)
	}

	// Recreate branch name at the parent.
	err = createBookmark(branchName)
	if err != nil {
		return fmt.Errorf("creating bookmark for branch %q at the parent: %w", branchName, err)
	}

	// Apply the patch.
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git apply %s", tmpFile.Name()),
	)
	if err != nil {
		return fmt.Errorf("applying patch: %w", err)
	}

	// Commit.
	err = commitAll(commitMessage)
	if err != nil {
		return err
	}

	// Checkout the original branch.
	if branchName != currBranch {
		err = updateRev(currBranch, nil)
		if err != nil {
			return fmt.Errorf("checking out original branch %q: %w", currBranch, err)
		}
	}

	return nil
}

func getCommitToStartPatchAtAndCommitMessage(
	repoData *git.RepoData,
	targetBranchName string,
	force bool,
) (string, string, error) {
	node, ok := repoData.BranchNameToNode[targetBranchName]
	if !ok {
		return "", "", fmt.Errorf("missing node for branch %q", targetBranchName)
	}

	parentBranch := node.BranchParent
	if parentBranch == nil {
		return "", "", fmt.Errorf("missing parent branch for branch %q", targetBranchName)
	}

	originNode, ok := repoData.BranchNameToNode["origin/"+targetBranchName]
	if !ok || force {
		// If no origin/BRANCHNAME local branch, branch hasn't been pushed yet, so all commits on the branch are safe to squash.
		commitToStartPatchAt := parentBranch.CommitMetadata.ShortCommitHash
		commitMessage := node.CommitMetadata.BranchDescription.Title
		return commitToStartPatchAt, commitMessage, nil
	}

	if originNode.CommitMetadata.ShortCommitHash != node.CommitMetadata.ShortCommitHash {
		_, err := shell.Run(
			shell.Opt{},
			fmt.Sprintf(
				"git merge-base --is-ancestor %s %s",
				originNode.CommitMetadata.ShortCommitHash,
				node.CommitMetadata.ShortCommitHash,
			),
		)
		isOriginAncestorOfBranch := err == nil

		if isOriginAncestorOfBranch {
			// If origin/BRANCHNAME local branch does not point to BRANCHNAME and is an ancestor of BRANCHNAME,
			// there are local commits that haven't been pushed yet that are safe to squash.
			commitToStartPatchAt := originNode.CommitMetadata.ShortCommitHash
			prettyFormat := "%s"
			out, err := shell.Run(
				shell.Opt{},
				fmt.Sprintf(
					"git log --pretty=format:%s --reverse %s..%s",
					prettyFormat,
					commitToStartPatchAt,
					node.CommitMetadata.ShortCommitHash,
				),
			)
			if err != nil {
				return "", "", fmt.Errorf("running git log: %w", err)
			}
			commitMessage := strings.TrimSpace(out)
			return commitToStartPatchAt, commitMessage, nil
		}
		// If origin/BRANCHNAME local branch exists but is not an ancestor of BRANCHNAME,
		// there must have been a previous squash so this origin/BRANCHNAME local branch can be ignored
		// and we can squash the entire branch.
		commitToStartPatchAt := parentBranch.CommitMetadata.ShortCommitHash
		commitMessage := node.CommitMetadata.BranchDescription.Title
		return commitToStartPatchAt, commitMessage, nil
	}

	// Else, all commits have been pushed, so it is not safe to squash.
	return "", "", fmt.Errorf(
		"not squashing since all commits on the branch have already been pushed. pass -f to force a squash",
	)
}
