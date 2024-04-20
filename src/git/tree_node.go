package git

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
