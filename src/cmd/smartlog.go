package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

func newSmartlogCmd() *cobra.Command {
	var showTime bool
	var cmd = &cobra.Command{
		Use:     "smartlog [-t time]",
		Short:   "Displays a smartlog: a sparse graph of commits relevant to you.",
		Long:    "Displays a smartlog of branches: a sparse graph of commits relevant to you. Branches are collapsed into single entries in the graph. Similar to `git log '--branches=*' --graph --decorate --oneline --simplify-by-decoration`.",
		Aliases: []string{"sl"},
		RunE: func(_ *cobra.Command, args []string) error {
			return runSmartlog(args, showTime)
		},
	}
	cmd.Flags().BoolVarP(&showTime, "time", "t", false, "Show time taken")
	return cmd
}

func runSmartlog(args []string, showTime bool) error {
	startTime := time.Now()
	repoData, err := git.NewRepoData()
	if err != nil {
		return err
	}
	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}
	// TODO: show commit even if head has no branch
	err = printNodeChildren(currBranch, repoData.BranchRootNode, "" /* prefix */)
	if err != nil {
		return err
	}

	elapsed := time.Since(startTime)
	if showTime {
		fmt.Printf("Finished in %.2f s.\n", elapsed.Seconds())
	}
	return nil
}

// Adapted from https://github.com/reydanro/git-smartlog
func printNodeChildren(currBranch string, node *git.TreeNode, prefix string) error {
	mainGraphConnector := ""
	children := sortedChildren(node)
	for i, child := range children {
		newPrefix := prefix + mainGraphConnector
		if i > 0 {
			newPrefix += " "
		}
		err := printNodeChildren(currBranch, child, newPrefix)
		if err != nil {
			return fmt.Errorf("printing node children: %w", err)
		}

		summary, err := getNodeSummary(child, currBranch)
		if err != nil {
			return fmt.Errorf("getting node summary: %w", err)
		}

		// First line
		var bullet string
		if child.CommitMetadata.IsHead {
			bullet = "*"
		} else {
			bullet = "o"
		}
		var graph string
		if i == 0 {
			graph = mainGraphConnector + bullet
		} else {
			graph = mainGraphConnector + " " + bullet
		}
		fmt.Println(prefix + graph + "  " + summary[0])

		// Update the connector character. Use ":" if parent is the root node.
		graphConnector := "|"
		if node.CommitMetadata == nil {
			graphConnector = ":"
		}
		if i == 0 {
			mainGraphConnector = graphConnector
		}

		// Second line
		if i == 0 {
			graph = mainGraphConnector
		} else {
			graph = mainGraphConnector + "/ "
		}
		fmt.Println(prefix + graph + "  " + summary[1])

		// Update the connector character.
		if i > 0 {
			mainGraphConnector = graphConnector
		}

		// Spacing to parent node
		if i < len(children)-1 {
			graph = mainGraphConnector
		} else {
			graph = graphConnector
		}
		fmt.Println(prefix + graph)
	}
	return nil
}

// Sort children by commit time.
// This means older commits will be shown earlier in the graph (further from the parent).
func sortedChildren(node *git.TreeNode) []*git.TreeNode {
	children := make([]*git.TreeNode, len(node.BranchChildren))
	copy(children, maps.Values(node.BranchChildren))
	sort.Slice(children, func(i, j int) bool {
		if children[i].CommitMetadata.IsAncestorOfMaster() {
			// Master should appear at depth 0.
			return true
		}
		if children[j].CommitMetadata.IsAncestorOfMaster() {
			// Master should appear at depth 0.
			return false
		}
		return children[i].CommitMetadata.Timestamp < children[j].CommitMetadata.Timestamp
	})
	return children
}

func getNodeSummary(node *git.TreeNode, currBranch string) ([]string, error) {
	commitMetadata := node.CommitMetadata
	if commitMetadata == nil {
		return nil, fmt.Errorf("missing CommitMetadata")
	}

	// First line
	var firstLine string
	var sha string
	if commitMetadata.IsHead {
		sha = color.MagentaString(commitMetadata.ShortCommitHash)
	} else {
		sha = color.YellowString(commitMetadata.ShortCommitHash)
	}
	firstLine += sha + "  "
	firstLine += commitMetadata.Author + "  "
	branchNames := commitMetadata.CleanedBranchNames()
	if len(branchNames) > 0 {
		firstLine += color.GreenString("(")
		for i, name := range branchNames {
			var comma string
			if i != len(branchNames)-1 {
				comma = ", "
			}
			if name == currBranch {
				firstLine += color.New(color.FgGreen, color.Bold).Sprintf("%s%s", name, comma)
			} else {
				firstLine += color.GreenString("%s%s", name, comma)
			}
		}
		firstLine += color.GreenString(")  ")
	}
	prURL, prURLText := commitMetadata.PRURL()
	if prURL != "" && prURLText != "" {
		firstLine += color.New(color.Bold).Sprintf("%s  ", util.Linkify(prURLText, prURL))
	}
	firstLine += commitMetadata.TimestampRelative

	// Second line
	var title string
	if commitMetadata.BranchDescription != nil {
		title = commitMetadata.BranchDescription.Title
	} else {
		title = commitMetadata.Title
	}
	secondLine := title

	return []string{firstLine, secondLine}, nil
}
