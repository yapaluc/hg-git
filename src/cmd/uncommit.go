package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/shell"

	"github.com/spf13/cobra"
)

const uncommitPatchFilePrefix = "PATCH_FILE_"

func newUncommitCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "uncommit",
		Short: "Uncommit the current branch.",
		Long:  "Uncommit the current branch so that all of its changes are unstaged.",
		Args:  cobra.NoArgs,
		RunE:  runUncommit,
	}
	return cmd
}

func runUncommit(_ *cobra.Command, args []string) error {
	repoData, err := git.NewRepoData()
	if err != nil {
		return err
	}

	branchName, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}

	node, ok := repoData.BranchNameToNode[branchName]
	if !ok {
		return fmt.Errorf("missing node for branch %q", branchName)
	}

	parentBranch := node.BranchParent
	if parentBranch == nil {
		return fmt.Errorf("missing parent branch for branch %q", branchName)
	}
	parentCommitHash := parentBranch.CommitMetadata.CommitHash

	// Prepare patch file.
	tmpDir := os.TempDir()

	tmpFileName := fmt.Sprintf("%s%s_", uncommitPatchFilePrefix, branchName)
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
			parentCommitHash,
			tmpFile.Name(),
		),
	)
	if err != nil {
		return fmt.Errorf("getting patch: %w", err)
	}

	// Checkout parent branch/commit.
	err = updateRev(parentCommitHash, nil)
	if err != nil {
		return fmt.Errorf("checking out parent branch/commit %q: %w", parentCommitHash, err)
	}

	// Apply the patch.
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git apply %s", tmpFile.Name()),
	)
	if err != nil {
		return fmt.Errorf("applying patch: %w", err)
	}

	// Delete the old branch
	err = deleteBranches([]string{branchName})
	if err != nil {
		return fmt.Errorf("deleting bookmark for branch %q: %w", branchName, err)
	}

	return nil
}
