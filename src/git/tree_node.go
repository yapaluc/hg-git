package git

import "fmt"

type TreeNode struct {
	CommitMetadata *commitMetadata
	BranchParent   *TreeNode
	BranchChildren map[string]*TreeNode
}

func (t *TreeNode) String() string {
	if t == nil {
		return "TreeNode<nil>"
	}
	return fmt.Sprintf("TreeNode<%s>", t.CommitMetadata.CommitHash)
}

func (t *TreeNode) addBranchParent(parent *TreeNode) error {
	if parent == nil {
		return fmt.Errorf("received nil parent when adding branch parent to %s", t)
	}
	if t == nil {
		return fmt.Errorf("received nil TreeNode child when adding branch parent %s", parent)
	}
	t.BranchParent = parent
	parent.BranchChildren[t.CommitMetadata.CommitHash] = t
	return nil
}

func newTreeNodeWithCommitHash(commitHash string) *TreeNode {
	return &TreeNode{
		CommitMetadata: &commitMetadata{CommitHash: commitHash},
		BranchChildren: make(map[string]*TreeNode),
	}
}
