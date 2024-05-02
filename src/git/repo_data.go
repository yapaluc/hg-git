package git

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yapaluc/hg-git/src/shell"
	"github.com/yapaluc/hg-git/src/util"
)

type RepoData struct {
	MasterBranch   string
	BranchRootNode *TreeNode
	// Commit hash to node. Each node is unique.
	CommitHashToNode map[string]*TreeNode
	// Branch name to node. Nodes may be duplicated.
	BranchNameToNode map[string]*TreeNode
}

type repoDataParams struct {
	IncludeCommitMetadata    bool
	IncludeBranchDescription bool
}

type RepoDataOption func(params *repoDataParams)

func RepoDataIncludeCommitMetadata(params *repoDataParams) {
	params.IncludeCommitMetadata = true
}

func RepoDataIncludeBranchDescription(params *repoDataParams) {
	params.IncludeBranchDescription = true
}

func NewRepoData(opts ...RepoDataOption) (*RepoData, error) {
	params := repoDataParams{}
	for _, opt := range opts {
		opt(&params)
	}
	repoData := &RepoData{
		BranchRootNode: &TreeNode{
			BranchChildren: make(map[string]*TreeNode),
		},
		CommitHashToNode: make(map[string]*TreeNode),
		BranchNameToNode: make(map[string]*TreeNode),
	}

	// Find master branch.
	masterBranch, err := GetMasterBranch()
	if err != nil {
		return nil, fmt.Errorf("getting master branch: %w", err)
	}
	repoData.MasterBranch = masterBranch

	// Build branch graph.
	err = repoData.buildBranchGraph()
	if err != nil {
		return nil, fmt.Errorf("building branch graph: %w", err)
	}

	// Add commit metadata.
	if params.IncludeCommitMetadata {
		err = repoData.addCommitMetadata()
		if err != nil {
			return nil, fmt.Errorf("adding commit metadata: %w", err)
		}
	}

	// Add branch description.
	if params.IncludeBranchDescription {
		err = repoData.addBranchDescription()
		if err != nil {
			return nil, fmt.Errorf("adding branch metadata: %w", err)
		}
	}

	return repoData, nil
}

func (rd *RepoData) addCommitMetadata() error {
	return populateCommitMetadata(rd.CommitHashToNode, rd.MasterBranch)
}

// NOTE: This only works for up to 29 branches. This is a limitation of `git show-branch`.
func (rd *RepoData) buildBranchGraph() error {
	// Find all the branches and their descendants.
	// Explanation of git show-branch: https://wincent.com/wiki/Understanding_the_output_of_%22git_show-branch%22
	output, err := shell.Run(
		shell.Opt{},
		"git show-branch --no-color",
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
		// Register a nodefor the branch and link it to the root node.
		r := regexp.MustCompile(`^\[(?P<commitref>.+)\] .*$`)
		match, err := util.RegexNamedMatches(r, split[0])
		if err != nil {
			return fmt.Errorf("extracting short commit hash: %w", err)
		}
		node, err := rd.registerNode(match["commitref"])
		if err != nil {
			return fmt.Errorf("registering node for %q: %w", match["commitref"], err)
		}
		node.addBranchParent(rd.BranchRootNode)
		return nil
	}

	prelude := strings.Split(strings.Trim(split[0], "\n"), "\n")
	body := strings.Split(strings.Trim(split[1], "\n"), "\n")

	// Find the position of each branch name and register a node for each branch name.
	branchNameToPosition := make(map[string]int)
	r = regexp.MustCompile(`^\s*[*!]\s*\[(?P<branch>.*)\] .*$`)
	for i, line := range prelude {
		match, err := util.RegexNamedMatches(r, line)
		if err != nil {
			return fmt.Errorf("extracting branch name: %w", err)
		}
		branchNameToPosition[match["branch"]] = i

		_, err = rd.registerNode(match["branch"])
		if err != nil {
			return fmt.Errorf("registering node for %q: %w", match["branch"], err)
		}
	}

	// Process each branch to find its branch parent.
	r = regexp.MustCompile(`^(?P<markers>[\s*+-]*) \[(?P<commitref>.+)\] .*$`)
	for branchName, position := range branchNameToPosition {
		// skip master branch - it is processed separately at the end
		if branchName == rd.MasterBranch {
			continue
		}

		node := rd.BranchNameToNode[branchName]
		foundBranchItself := false
	innerLoop:
		// traverse sequentially to find the first parent branch of the current branch
		for _, line := range body {
			match, err := util.RegexNamedMatches(r, line)
			if err != nil {
				return fmt.Errorf("extracting markers and hash: %w", err)
			}
			commitRef := match["commitref"]
			char := match["markers"][position]
			// current line is not relevant to this branch
			if char == ' ' {
				continue
			}
			// skip the branch itself
			if commitRef == branchName {
				foundBranchItself = true
				continue
			}
			if !foundBranchItself {
				// do not pick anything until we passed the branch itself (this is for cases when a branch points at master)
				continue
			}
			// pick master
			if commitRef == rd.MasterBranch {
				node.addBranchParent(rd.BranchNameToNode[rd.MasterBranch])
				break innerLoop
			}
			// pick an ancestor of master
			if strings.HasPrefix(commitRef, rd.MasterBranch+"~") ||
				strings.HasPrefix(commitRef, rd.MasterBranch+"^") {
				// special case: create a node for an ancestor of master
				parentNode, err := rd.registerNode(commitRef)
				if err != nil {
					return fmt.Errorf("registering node for %q: %w", commitRef, err)
				}
				parentNode.CommitMetadata.IsPartOfMaster = true
				node.addBranchParent(parentNode)
				break innerLoop
			}
			// skip ancestors of other branches
			if strings.Contains(commitRef, "~") || strings.HasSuffix(commitRef, "^") {
				continue
			}
			// else, must be another branch
			node.addBranchParent(rd.BranchNameToNode[commitRef])
			break innerLoop
		}
	}

	// finally, process master branch to connect relevant ancestors of master to the master node
	node := rd.BranchNameToNode[rd.MasterBranch]
	for _, line := range body {
		match, err := util.RegexNamedMatches(r, line)
		if err != nil {
			return fmt.Errorf("extracting markers and hash: %w", err)
		}
		commitRef := match["commitref"]
		char := match["markers"][branchNameToPosition[rd.MasterBranch]]
		// current line is not relevant to this branch
		if char == ' ' {
			continue
		}
		// skip the branch itself
		if commitRef == rd.MasterBranch {
			continue
		}
		// if ancestor has been registered, add it as branch parent
		if masterAncestor, ok := rd.BranchNameToNode[commitRef]; ok {
			node.addBranchParent(masterAncestor)
			node = masterAncestor
		}
	}

	// nodes with no branch parent should point at the root node
	for _, node := range rd.CommitHashToNode {
		if node.BranchParent == nil {
			node.addBranchParent(rd.BranchRootNode)
		}
	}

	return nil
}

// commitRef is a branch name or a string parseable by `git rev-parse`.
func (rd *RepoData) registerNode(commitRef string) (*TreeNode, error) {
	commitHash, err := ResolveCommitRef(commitRef)
	if err != nil {
		return nil, fmt.Errorf("resolving commit ref %q: %w", commitRef, err)
	}

	if _, ok := rd.CommitHashToNode[commitHash]; !ok {
		// Create node with minimal commitMetadata.
		// Node may exist already in case of multiple branches pointing at the same commit.
		rd.CommitHashToNode[commitHash] = newTreeNodeWithCommitHash(commitHash)
	}
	rd.BranchNameToNode[commitRef] = rd.CommitHashToNode[commitHash]
	return rd.CommitHashToNode[commitHash], nil
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
