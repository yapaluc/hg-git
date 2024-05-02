package git

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/samber/lo"
	"github.com/yapaluc/hg-git/src/github"
	"github.com/yapaluc/hg-git/src/shell"
	"github.com/yapaluc/hg-git/src/util"
)

const endBodyMarker = "__ENDBODY__"

type commitMetadata struct {
	// Minimal fields populated upon construction
	CommitHash string

	// Fields supplemented by RepoData
	IsPartOfMaster    bool
	BranchDescription *BranchDescription

	// Fields supplemented later by this file
	ShortCommitHash   string
	Author            string
	TimestampRelative string
	Timestamp         int64
	BranchNames       []string
	IsHead            bool
	Title             string
	IsMaster          bool
}

// Mutates the commitMetadata of each node in the given map.
func populateCommitMetadata(commitHashToNode map[string]*TreeNode, masterBranch string) error {
	revList, err := getRevList(lo.Keys(commitHashToNode))
	if err != nil {
		return fmt.Errorf("getting rev list: %w", err)
	}

	for i := 0; i < len(revList); i++ {
		start := i
		for revList[i] != endBodyMarker {
			i++
		}
		commitMetadata, err := newCommitMetadata(revList, start, masterBranch)
		if err != nil {
			return fmt.Errorf("parsing commit metadata: %w", err)
		}
		node := commitHashToNode[commitMetadata.CommitHash]
		if node == nil {
			return fmt.Errorf(
				"failed to find node for commit hash %s: %w",
				commitMetadata.CommitHash,
				err,
			)
		}
		commitMetadata.IsPartOfMaster = node.CommitMetadata.IsPartOfMaster
		node.CommitMetadata = commitMetadata
	}
	return nil
}

// Format is:
//
//	commit <commit hash> <children hashes separated by spaces>
//	<short commit hash>
//	<author name>
//	<relative commit time>
//	<commit timestamp>
//	<comma-separated branch names>
//	<commit title>
//	<multi-line commit body>
//	__ENDBODY__
func getRevList(commitHashes []string) ([]string, error) {
	prettyFormat := "%h%n%an%n%cr%n%ct%n%D%n%s%n%b%n" + endBodyMarker
	lines, err := shell.RunAndCollectLines(shell.Opt{}, fmt.Sprintf(
		// --no-walk=sorted limits the results to only the given commits.
		// By default, `git rev-list`` includes all commits reachable from the given commits.
		"git rev-list --pretty=format:%s %s --no-walk=sorted",
		prettyFormat,
		strings.Join(commitHashes, " "),
	))
	if err != nil {
		return nil, fmt.Errorf("getting rev list: %w", err)
	}
	return lines, nil
}

func newCommitMetadata(
	lines []string,
	start int,
	masterBranch string,
) (*commitMetadata, error) {
	firstLineHashes := strings.Split(lines[start], " ")
	timestamp, err := strconv.ParseInt(lines[start+4], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("converting timestamp to int: %w", err)
	}
	var branchNames []string
	var isHead bool
	for _, branchName := range strings.Split(lines[start+5], ", ") {
		if branchName == "" {
			continue
		}
		if strings.HasPrefix(branchName, "HEAD -> ") {
			isHead = true
			branchName = strings.TrimPrefix(branchName, "HEAD -> ")
		} else if branchName == "HEAD" {
			isHead = true
			continue
		}
		branchNames = append(branchNames, branchName)
	}
	return &commitMetadata{
		CommitHash:        firstLineHashes[1], // first element is "commit" string literal
		ShortCommitHash:   lines[start+1],
		Author:            lines[start+2],
		TimestampRelative: lines[start+3],
		Timestamp:         timestamp,
		BranchNames:       branchNames,
		IsHead:            isHead,
		Title:             lines[start+6],
		IsMaster:          lo.Contains(branchNames, masterBranch),
	}, nil
}

func (cm *commitMetadata) CleanedBranchNames() []string {
	return lo.Filter(cm.BranchNames, func(branchName string, _ int) bool {
		return !strings.HasPrefix(branchName, "refs/branchless/") &&
			!strings.HasPrefix(branchName, "origin/") &&
			!strings.HasPrefix(branchName, "tag: ")
	})
}

// Returns true if the commit is master or an ancestor of master.
// This is only valid if this commit is part of the branch graph.
func (cm *commitMetadata) IsEffectiveMaster() bool {
	return cm.IsMaster || len(cm.CleanedBranchNames()) == 0
}

func (cm *commitMetadata) IsAncestorOfMaster() bool {
	return cm.IsMaster || cm.IsPartOfMaster
}

func (cm *commitMetadata) PRURL() (string, string) {
	if cm.BranchDescription != nil && cm.BranchDescription.PrURL != "" {
		prURL := cm.BranchDescription.PrURL
		linkText := github.PRStrFromPRURL(prURL)
		return prURL, linkText
	}

	r := regexp.MustCompile(` \(#(?P<prnum>\d+)\)$`)
	match, err := util.RegexNamedMatches(r, cm.Title)
	if err != nil {
		return "", ""
	}

	prNum, err := strconv.Atoi(match["prnum"])
	if err != nil {
		// Suppress this error in case there are weird titles.
		return "", ""
	}
	remoteURL, err := GetRemoteURL()
	if err != nil {
		// Suppress this error in case there is no remote.
		return "", ""
	}
	prURL := fmt.Sprintf("%s/pull/%d", remoteURL, prNum)
	linkText := fmt.Sprintf("#%d", prNum)
	return prURL, linkText
}
