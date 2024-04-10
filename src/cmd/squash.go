package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/spf13/cobra"
)

const squashPatchFilePrefix = "PATCH_FILE_"

func newSquashCmd() *cobra.Command {
	var force bool
	var cmd = &cobra.Command{
		Use:     "squash [branch name] [-f force]",
		Short:   "Squash the current branch into one commit.",
		Long:    "Squash the current branch into one commit. Works across merges and in the middle of a stack.",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"sq"},
		RunE: func(_ *cobra.Command, args []string) error {
			return runSquash(args, force)
		},
	}
	cmd.Flags().
		BoolVarP(&force, "force", "f", false, "Force the squash through, despite the fact that commits have been pushed to the remote or that the branch is in the middle of a stack")
	return cmd
}

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

	lastBranch, err := runSquashOnBranchAndDescendants(repoData, branchName, force)
	if err != nil {
		return fmt.Errorf("running squash on branch %q and its descendants: %w", branchName, err)
	}

	// Checkout the original branch.
	if lastBranch != currBranch {
		err = updateRev(currBranch, nil)
		if err != nil {
			return fmt.Errorf("checking out original branch %q: %w", currBranch, err)
		}
	}

	return nil
}

// Implements a depth-first traversal to squash a branch and its descendants.
// Returns the name of the last branch processed.
func runSquashOnBranchAndDescendants(
	repoData *git.RepoData,
	branchName string,
	force bool,
) (string, error) {
	node, ok := repoData.BranchNameToNode[branchName]
	if !ok {
		return "", fmt.Errorf("missing node for branch %q", branchName)
	}

	if len(node.BranchChildren) > 0 && !force {
		return "", fmt.Errorf(
			"not squashing since branch %q has descendant branches. pass -f to force a squash of branch %q and its descendants",
			branchName,
			branchName,
		)
	}

	// Squash current branch
	lastBranchName := branchName
	err := runSquashOnBranch(repoData, branchName, force)
	if err != nil {
		return "", fmt.Errorf("running squash on branch %q: %w", branchName, err)
	}

	// Recurse on children
	for _, childNode := range node.BranchChildren {
		childBranchName := childNode.CommitMetadata.CleanedBranchNames()[0]
		lastBranchNameProcessedInChildSubtree, err := runSquashOnBranchAndDescendants(
			repoData,
			childBranchName,
			force,
		)
		if err != nil {
			return "", fmt.Errorf(
				"running squash on child branch %q of %q: %w",
				childBranchName,
				branchName,
				err,
			)
		}
		lastBranchName = lastBranchNameProcessedInChildSubtree
	}

	return lastBranchName, nil
}

func runSquashOnBranch(repoData *git.RepoData, branchName string, force bool) error {
	// Checkout branch.
	err := updateRev(branchName, nil)
	if err != nil {
		return fmt.Errorf("checking out branch %q: %w", branchName, err)
	}

	// Get squash details.
	squashDetails, err := getSquashDetails(repoData, branchName, force)
	if err != nil {
		return fmt.Errorf("getting squash details: %w", err)
	}

	// Prepare patch file.
	tmpDir := os.TempDir()

	tmpFileName := fmt.Sprintf("%s%s_", squashPatchFilePrefix, branchName)
	tmpFileName = strings.ReplaceAll(tmpFileName, string(os.PathSeparator), "-")
	tmpFile, err := os.CreateTemp(tmpDir, tmpFileName)
	if err != nil {
		return fmt.Errorf("creating temp file for patch: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Get patch.
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf(
			"git diff-index %s --binary > %s",
			squashDetails.commitToCalculatePatchFrom,
			tmpFile.Name(),
		),
	)
	if err != nil {
		return fmt.Errorf("getting patch: %w", err)
	}

	// Checkout parent branch.
	err = updateRev(squashDetails.branchNameToPatchAt, nil)
	if err != nil {
		return fmt.Errorf(
			"checking out parent branch %q: %w",
			squashDetails.branchNameToPatchAt,
			err,
		)
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
	err = commitAll(squashDetails.commitMessage)
	if err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	return err
}

type squashDetails struct {
	commitToCalculatePatchFrom string
	branchNameToPatchAt        string
	commitMessage              string
}

func getSquashDetails(
	repoData *git.RepoData,
	targetBranchName string,
	force bool,
) (squashDetails, error) {
	node := repoData.BranchNameToNode[targetBranchName]
	parentBranch := node.BranchParent
	if parentBranch == nil {
		return squashDetails{}, fmt.Errorf("missing parent branch for branch %q", targetBranchName)
	}

	parentBranchName := parentBranch.CommitMetadata.CleanedBranchNames()[0]

	originNode, ok := repoData.BranchNameToNode["origin/"+targetBranchName]
	if !ok || force {
		// If no origin/BRANCHNAME local branch, branch hasn't been pushed yet, so all commits on the branch are safe to squash.
		commitToStartPatchAt := parentBranch.CommitMetadata.ShortCommitHash
		commitMessage := node.CommitMetadata.BranchDescription.Title
		return squashDetails{
			commitToCalculatePatchFrom: commitToStartPatchAt,
			branchNameToPatchAt:        parentBranchName,
			commitMessage:              commitMessage,
		}, nil
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
		result, err := shell.Run(
			shell.Opt{},
			fmt.Sprintf(
				"git log --merges %s..%s",
				originNode.CommitMetadata.ShortCommitHash,
				node.CommitMetadata.ShortCommitHash,
			),
		)
		if err != nil {
			return squashDetails{}, fmt.Errorf(
				"calling git log to check for merge commits",
			)
		}
		hasMergeCommits := len(result) > 0

		if hasMergeCommits {
			return squashDetails{}, fmt.Errorf(
				"not squashing since there are merge commits on the branch after the commit has been pushed. pass -f to force a squash",
			)
		}

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
				return squashDetails{}, fmt.Errorf("running git log: %w", err)
			}
			commitMessage := strings.TrimSpace(out)
			return squashDetails{
				commitToCalculatePatchFrom: commitToStartPatchAt,
				branchNameToPatchAt:        parentBranchName,
				commitMessage:              commitMessage,
			}, nil
		}
		// If origin/BRANCHNAME local branch exists but is not an ancestor of BRANCHNAME,
		// there must have been a previous squash so this origin/BRANCHNAME local branch can be ignored
		// and we can squash the entire branch.
		commitToStartPatchAt := parentBranch.CommitMetadata.ShortCommitHash
		commitMessage := node.CommitMetadata.BranchDescription.Title
		return squashDetails{
			commitToCalculatePatchFrom: commitToStartPatchAt,
			branchNameToPatchAt:        parentBranchName,
			commitMessage:              commitMessage,
		}, nil
	}

	// Else, all commits have been pushed, so it is not safe to squash.
	return squashDetails{}, fmt.Errorf(
		"not squashing since all commits on the branch have already been pushed. pass -f to force a squash",
	)
}
