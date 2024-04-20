package git

import (
	"container/list"
	"fmt"
	"regexp"
	"strings"

	"github.com/yapaluc/hg-git/src/shell"
	"github.com/yapaluc/hg-git/src/util"
)

type RepoData struct {
	BranchRootNode        *TreeNode
	CommitHashToNode      map[string]*TreeNode
	ShortCommitHashToNode map[string]*TreeNode
	BranchNameToNode      map[string]*TreeNode
	MasterBranch          string
}

func NewRepoData() (*RepoData, error) {
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
	err = repoData.buildCommitGraph()
	if err != nil {
		return nil, fmt.Errorf("building commit graph: %w", err)
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

func (rd *RepoData) buildCommitGraph() error {
	commitMetadatas, err := newCommitMetadatas(rd.MasterBranch)
	if err != nil {
		return fmt.Errorf("creating commit metadatas: %w", err)
	}
	for _, commitMetadata := range commitMetadatas {
		rd.addCommit(commitMetadata)
	}
	return nil
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
