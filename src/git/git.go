package git

import (
	"container/list"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/yapaluc/hg-git/src/github"
	"github.com/yapaluc/hg-git/src/shell"
	"github.com/yapaluc/hg-git/src/util"

	"github.com/alessio/shellescape"
	"github.com/samber/lo"
)

const endBodyMarker = "__ENDBODY__"

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

	return strings.TrimPrefix(strings.TrimSpace(branch), "origin/"), nil
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
func getRevList() ([]string, error) {
	prettyFormat := "%h%n%an%n%cr%n%ct%n%D%n%s%n%b%n" + endBodyMarker
	lines, err := shell.RunAndCollectLines(shell.Opt{}, fmt.Sprintf(
		"git rev-list --children --all --pretty=format:%s",
		prettyFormat,
	))
	if err != nil {
		return nil, fmt.Errorf("getting rev list: %w", err)
	}
	return lines, nil
}

type commitMetadata struct {
	CommitHash        string
	ShortCommitHash   string
	childrenHashes    []string
	Author            string
	TimestampRelative string
	Timestamp         int
	BranchNames       []string
	IsHead            bool
	Title             string
	body              string
	BranchDescription *BranchDescription
	IsMaster          bool
	IsPartOfMaster    bool
}

func (cm *commitMetadata) CleanedBranchNames() []string {
	return lo.Filter(cm.BranchNames, func(branchName string, _ int) bool {
		return isLocalBranch(branchName)
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

type TreeNode struct {
	CommitMetadata *commitMetadata
	parents        map[string]*TreeNode
	BranchParent   *TreeNode
	children       map[string]*TreeNode
	BranchChildren map[string]*TreeNode
}

func (t *TreeNode) addBranchParent(parent *TreeNode) {
	t.BranchParent = parent
	parent.BranchChildren[t.CommitMetadata.CommitHash] = t
}

func newTreeNode(commitMetadata *commitMetadata) *TreeNode {
	return &TreeNode{
		CommitMetadata: commitMetadata,
		parents:        make(map[string]*TreeNode),
		children:       make(map[string]*TreeNode),
		BranchChildren: make(map[string]*TreeNode),
	}
}

type RepoData struct {
	BranchRootNode        *TreeNode
	CommitHashToNode      map[string]*TreeNode
	ShortCommitHashToNode map[string]*TreeNode
	BranchNameToNode      map[string]*TreeNode
	MasterBranch          string
}

func NewRepoData() (*RepoData, error) {
	revList, err := getRevList()
	if err != nil {
		return nil, fmt.Errorf("getting rev list: %w", err)
	}

	repoData := &RepoData{
		BranchRootNode: &TreeNode{
			BranchChildren: make(map[string]*TreeNode),
		},
		CommitHashToNode:      make(map[string]*TreeNode),
		ShortCommitHashToNode: make(map[string]*TreeNode),
		BranchNameToNode:      make(map[string]*TreeNode),
	}

	// Find master branch.
	masterBranch, err := GetMasterBranch()
	if err != nil {
		return nil, fmt.Errorf("getting master branch: %w", err)
	}
	repoData.MasterBranch = masterBranch

	// Build commit graph.
	for i := 0; i < len(revList); i++ {
		start := i
		for revList[i] != endBodyMarker {
			i++
		}
		commitMetadata, err := newCommitMetadata(revList, start, i, repoData.MasterBranch)
		if err != nil {
			return nil, fmt.Errorf("parsing commit metadata: %w", err)
		}
		repoData.addCommit(commitMetadata)
	}

	// Build branch graph.
	err = repoData.buildBranchGraph()
	if err != nil {
		return nil, fmt.Errorf("building branch graph: %w", err)
	}

	// Add branch description.
	err = repoData.addBranchDescription()
	if err != nil {
		return nil, fmt.Errorf("adding branch metadata: %w", err)
	}

	return repoData, nil
}

func (rd *RepoData) addCommit(commitMetadata *commitMetadata) {
	// Create or update the node. It may have been created earlier as a shell
	// when processing the parent.
	node, ok := rd.CommitHashToNode[commitMetadata.CommitHash]
	if ok {
		node.CommitMetadata = commitMetadata
	} else {
		node = newTreeNode(commitMetadata)
		rd.CommitHashToNode[commitMetadata.CommitHash] = node
	}
	rd.ShortCommitHashToNode[commitMetadata.ShortCommitHash] = node

	// Add branch names.
	for _, branchName := range commitMetadata.BranchNames {
		rd.BranchNameToNode[branchName] = node
	}

	// Create nodes for the children if they don't exist (real data will be added later).
	for _, childHash := range commitMetadata.childrenHashes {
		childNode, ok := rd.CommitHashToNode[childHash]
		if !ok {
			childNode = newTreeNode(nil /* commitMetadata */)
			rd.CommitHashToNode[childHash] = childNode
		}
		childNode.parents[commitMetadata.CommitHash] = node
		node.children[childHash] = childNode
	}
}

func (rd *RepoData) buildBranchGraph() error {
	// Find all the branches and their descendants.
	// Explanation of git show-branch: https://wincent.com/wiki/Understanding_the_output_of_%22git_show-branch%22
	output, err := shell.Run(
		shell.Opt{},
		"git show-branch --sha1-name --topo-order --no-color",
	)
	output = strings.TrimSpace(output)
	if err != nil {
		return fmt.Errorf("getting branch data: %w", err)
	}

	r := regexp.MustCompile(`(?m)^-+$`)
	split := r.Split(output, 2)

	if len(split) == 1 {
		// If there is only one branch, git changes the output format
		// and omits the prelude and column indicator.
		r := regexp.MustCompile(`^\[(?P<shorthash>[a-z0-9]+)\] .*$`)
		match, err := util.RegexNamedMatches(r, split[0])
		if err != nil {
			return fmt.Errorf("extracting short commit hash: %w", err)
		}
		node := rd.ShortCommitHashToNode[match["shorthash"]]
		node.addBranchParent(rd.BranchRootNode)
		return nil
	}

	prelude := strings.Split(strings.Trim(split[0], "\n"), "\n")
	body := strings.Split(strings.Trim(split[1], "\n"), "\n")

	var depthToBranchName []string
	r = regexp.MustCompile(`^\s*[*!]\s*\[(?P<branch>.*)\] .*$`)
	for _, line := range prelude {
		match, err := util.RegexNamedMatches(r, line)
		if err != nil {
			return fmt.Errorf("extracting branch name: %w", err)
		}
		depthToBranchName = append(depthToBranchName, match["branch"])
	}

	shortCommitHashToChildren := make(map[string]map[string]struct{})
	shortCommitHashToInDeg := make(map[string]int)
	allShortCommitHashes := make(map[string]struct{})
	branchNameToPrevCommitHash := make(map[string]string)
	shortCommitHashesInMaster := make(map[string]struct{})
	r = regexp.MustCompile(`^(?P<markers>[\s*+-]*) \[(?P<shorthash>[a-z0-9]+)\] .*$`)
	for _, line := range body {
		match, err := util.RegexNamedMatches(r, line)
		if err != nil {
			return fmt.Errorf("extracting markers and hash: %w", err)
		}
		shortCommitHash := match["shorthash"]
		allShortCommitHashes[shortCommitHash] = struct{}{}
		for i, char := range match["markers"] {
			if char == ' ' {
				continue
			}
			branchName := depthToBranchName[i]
			if prevCommitHash, ok := branchNameToPrevCommitHash[branchName]; ok {
				if _, found := shortCommitHashToChildren[shortCommitHash][prevCommitHash]; !found {
					if shortCommitHashToChildren[shortCommitHash] == nil {
						shortCommitHashToChildren[shortCommitHash] = make(map[string]struct{})
					}
					shortCommitHashToChildren[shortCommitHash][prevCommitHash] = struct{}{}
					shortCommitHashToInDeg[prevCommitHash]++
				}
			}
			branchNameToPrevCommitHash[branchName] = shortCommitHash

			if branchName == rd.MasterBranch {
				shortCommitHashesInMaster[shortCommitHash] = struct{}{}
			}
		}
	}

	// Run a topological search to tighten it up to direct parent-child relationships.
	q := list.New()
	type qElem struct {
		commitHash       string
		parentBranchNode *TreeNode
		parentNode       *TreeNode
		root             bool
	}
	for commitHash := range allShortCommitHashes {
		inDeg, ok := shortCommitHashToInDeg[commitHash]
		if !ok || inDeg == 0 {
			q.PushBack(qElem{
				commitHash:       commitHash,
				parentBranchNode: rd.BranchRootNode,
				parentNode:       rd.BranchRootNode,
				root:             true,
			})
		}
	}

	visitedShortCommitHashes := make(map[string]struct{})
	for q.Len() != 0 {
		elem := q.Front()
		q.Remove(elem)
		innerElem := elem.Value.(qElem)
		node := rd.ShortCommitHashToNode[innerElem.commitHash]
		commitHash := innerElem.commitHash
		if _, ok := visitedShortCommitHashes[commitHash]; ok {
			continue
		}
		visitedShortCommitHashes[commitHash] = struct{}{}

		var parentBranchNodeToPropagate *TreeNode
		if len(node.CommitMetadata.CleanedBranchNames()) > 0 || innerElem.root {
			parent := innerElem.parentBranchNode
			if innerElem.parentNode != innerElem.parentBranchNode {
				_, parentNodeIsInMaster := shortCommitHashesInMaster[innerElem.parentNode.CommitMetadata.ShortCommitHash]
				if parentNodeIsInMaster {
					// Use parent node instead of parent branch node if it is part of master.
					parent = innerElem.parentNode
					// Register that node as part of the graph.
					parent.addBranchParent(innerElem.parentBranchNode)
					// Set the flag
					parent.CommitMetadata.IsPartOfMaster = true
				}
			}
			node.addBranchParent(parent)
			parentBranchNodeToPropagate = node
		} else {
			// If this node doesn't represent a branch, propagate its parent node as the parent branch node.
			parentBranchNodeToPropagate = innerElem.parentBranchNode
		}
		for child := range shortCommitHashToChildren[commitHash] {
			shortCommitHashToInDeg[child]--
			if shortCommitHashToInDeg[child] == 0 {
				q.PushBack(qElem{
					commitHash:       child,
					parentBranchNode: parentBranchNodeToPropagate,
					parentNode:       node,
				})
			}
		}
	}

	return nil
}

func (rd *RepoData) addBranchDescription() error {
	lines, err := shell.RunAndCollectLines(
		shell.Opt{},
		`git config --get-regexp 'branch\.(.*)\.description'`,
	)
	if err != nil {
		// git config exits with code 1 if there are no branch descriptions
		return nil
	}

	r := regexp.MustCompile(`^branch\.(?P<branch>.*)\.description(?P<first_line>.*)$`)
	i := 0
	for i < len(lines) {
		start := i
		i++
		for i < len(lines) {
			if r.MatchString(lines[i]) {
				break
			}
			i++
		}
		match, err := util.RegexNamedMatches(r, lines[start])
		if err != nil {
			return fmt.Errorf("extracting branch and first line: %w", err)
		}
		node, ok := rd.BranchNameToNode[match["branch"]]
		if ok {
			node.CommitMetadata.BranchDescription = NewBranchDescription(
				match["first_line"],
				lines[start+1:i],
			)
		}
	}
	return nil
}

func newCommitMetadata(
	lines []string,
	start, end int,
	masterBranch string,
) (*commitMetadata, error) {
	firstLineHashes := strings.Split(lines[start], " ")
	timestamp, err := strconv.Atoi(lines[start+4])
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
		childrenHashes:    firstLineHashes[2:],
		Author:            lines[start+2],
		TimestampRelative: lines[start+3],
		Timestamp:         timestamp,
		BranchNames:       branchNames,
		IsHead:            isHead,
		Title:             lines[start+6],
		// Don't include end_line (END_BODY marker).
		body:     strings.Join(lines[start+7:end], "\n"),
		IsMaster: lo.Contains(branchNames, masterBranch),
	}, nil
}

func isLocalBranch(branchName string) bool {
	return !strings.HasPrefix(branchName, "refs/branchless/") &&
		!strings.HasPrefix(branchName, "origin/")
}
