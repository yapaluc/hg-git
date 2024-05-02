package git

type TreeNode struct {
	CommitMetadata *commitMetadata
	BranchParent   *TreeNode
	BranchChildren map[string]*TreeNode
}

func (t *TreeNode) addBranchParent(parent *TreeNode) {
	t.BranchParent = parent
	parent.BranchChildren[t.CommitMetadata.CommitHash] = t
}

func newTreeNodeWithCommitHash(commitHash string) *TreeNode {
	return &TreeNode{
		CommitMetadata: &commitMetadata{CommitHash: commitHash},
		BranchChildren: make(map[string]*TreeNode),
	}
}
