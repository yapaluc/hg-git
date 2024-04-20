package git

import (
	"fmt"
	"strings"

	"github.com/yapaluc/hg-git/src/shell"

	"github.com/alessio/shellescape"
	"github.com/samber/lo"
)

func GetCurrentBranch() (string, error) {
	currBranch, err := shell.Run(shell.Opt{}, "git branch --show-current")
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(currBranch), nil
}

// https://stackoverflow.com/a/45560221
func GetMasterBranch() (string, error) {
	branch, err := shell.Run(
		shell.Opt{},
		`git branch -a | awk '/remotes\/origin\/HEAD/ {print $NF}'`,
	)
	if err != nil {
		return "", fmt.Errorf("getting master branch name: %w", err)
	}
	candidateName := strings.TrimPrefix(strings.TrimSpace(branch), "origin/")
	if candidateName == "" {
		// https://stackoverflow.com/questions/28666357/how-to-get-default-git-branch#comment105620968_44750379
		return "", fmt.Errorf(
			"getting master branch name: remotes/origin/HEAD ref not found. try running `git remote set-head origin --auto` to sync the ref from upstream",
		)
	}
	return candidateName, nil
}

func GetRemoteURL() (string, error) {
	remoteURL, err := shell.Run(
		shell.Opt{},
		"git config --get remote.origin.url",
	)
	if err != nil {
		return "", fmt.Errorf("getting remote url: %w", err)
	}
	remoteURL = strings.TrimSpace(remoteURL)
	remoteURL = strings.TrimSuffix(remoteURL, ".git")
	remoteURL = strings.TrimSuffix(remoteURL, "/")

	return remoteURL, nil
}

func ResolveRev(rev string) (string, error) {
	if rev == ".^" {
		// Find the previous branch.
		repoData, err := NewRepoData()
		if err != nil {
			return "", err
		}

		currBranch, err := GetCurrentBranch()
		if err != nil {
			return "", err
		}
		rev = repoData.BranchNameToNode[currBranch].BranchParent.CommitMetadata.CommitHash
	}
	return rev, nil
}

// Resolves the given string to a branch name.
// The string can be a branch name itself.
// It can also be a commit hash pointing a commit with a branch name.
type branchNameResolution struct {
	BranchName string
	CommitHash string
}

func ResolveBranchName(rev string, excludeBranch *string) (*branchNameResolution, error) {
	if rev == "." {
		rev = "HEAD"
	}
	branches, err := shell.RunAndCollectLines(
		shell.Opt{},
		fmt.Sprintf(
			"git branch --points-at %s --format %s",
			shellescape.Quote(rev),
			shellescape.Quote("%(refname:short)"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("getting branch names of the rev %q: %w", rev, err)
	}
	if excludeBranch != nil {
		branches = lo.Filter(branches, func(branch string, _ int) bool {
			return branch != *excludeBranch
		})
	}
	if len(branches) == 0 {
		return &branchNameResolution{CommitHash: rev}, nil
	}
	if len(branches) == 1 || !lo.Contains(branches, rev) {
		return &branchNameResolution{BranchName: branches[0]}, nil
	}
	return &branchNameResolution{BranchName: rev}, nil
}
