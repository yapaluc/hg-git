package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/yapaluc/hg-git/src/shell"

	"github.com/spf13/cobra"
)

const patchPatchFilePrefix = "PATCH_FILE_"

func newPatchCmd() *cobra.Command {
	var force bool
	var cmd = &cobra.Command{
		Use:   "patch [rev] [-f force]",
		Short: "Patch the given rev as local uncommitted changes.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runPatch(args, force)
		},
	}
	cmd.Flags().
		BoolVarP(&force, "force", "f", false, "Force the patch through best-effort, even if some changes are rejected. Leaves rejected changes in .rej files.")
	return cmd
}

func runPatch(args []string, force bool) error {
	rev := args[0]
	err := runPatchOnRev(rev, force)
	if err != nil {
		return fmt.Errorf("running patch on rev %q: %w", rev, err)
	}
	return nil
}

func runPatchOnRev(rev string, force bool) error {
	// Prepare patch file.
	tmpDir := os.TempDir()

	tmpFileName := fmt.Sprintf("%s%s_", patchPatchFilePrefix, rev)
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
			"git diff --binary %s^ %s > %s",
			rev,
			rev,
			tmpFile.Name(),
		),
	)
	if err != nil {
		return fmt.Errorf("getting patch: %w", err)
	}

	// Apply the patch.
	rejectFlag := "--no-reject"
	if force {
		rejectFlag = "--reject"
	}
	_, err = shell.Run(
		shell.Opt{StreamOutputToStdout: true},
		fmt.Sprintf("git apply %s %s", tmpFile.Name(), rejectFlag),
	)
	if err != nil {
		return fmt.Errorf("applying patch: %w", err)
	}

	return nil
}
